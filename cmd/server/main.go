package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/asifthewebguy/webhostmanager-api/internal/audit"
	"github.com/asifthewebguy/webhostmanager-api/internal/auth"
	"github.com/asifthewebguy/webhostmanager-api/internal/config"
	"github.com/asifthewebguy/webhostmanager-api/internal/database"
	"github.com/asifthewebguy/webhostmanager-api/internal/health"
	"github.com/asifthewebguy/webhostmanager-api/internal/router"
	"github.com/asifthewebguy/webhostmanager-api/internal/scheduler"
	"github.com/asifthewebguy/webhostmanager-api/internal/server"
	"github.com/asifthewebguy/webhostmanager-api/internal/setup"
	"github.com/asifthewebguy/webhostmanager-api/internal/users"
	"github.com/asifthewebguy/webhostmanager-api/pkg/ratelimit"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}
	log.Info().Str("env", cfg.Server.Env).Msg("starting WebHostManager API")

	db, err := database.Connect(&cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	log.Info().Msg("database connected")

	if err := database.RunMigrations(cfg.Database.DSN, cfg.Database.MigrationsPath); err != nil {
		log.Fatal().Err(err).Msg("failed to run migrations")
	}
	log.Info().Msg("migrations applied")

	// Services
	authSvc   := auth.NewService(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiryHrs)
	auditSvc  := audit.NewService(db)
	usersSvc  := users.NewService(db)
	setupSvc  := setup.NewService(db, cfg.Auth.EncryptionKey)
	serverSvc := server.NewService(db, cfg.Auth.EncryptionKey)
	limiter   := ratelimit.NewLoginLimiter()

	// Seed the metrics cache on startup so the dashboard has data immediately.
	go func() {
		if err := serverSvc.CollectAndCache(); err != nil {
			log.Warn().Err(err).Msg("initial metrics collection failed")
		} else {
			log.Info().Msg("initial metrics cached")
		}
	}()

	// Scheduler: refresh metrics every 30 seconds.
	sched := scheduler.New()
	sched.Every30s(func() {
		if err := serverSvc.CollectAndCache(); err != nil {
			log.Warn().Err(err).Msg("scheduled metrics collection failed")
		}
	})
	sched.Start()
	defer sched.Stop()

	// Handlers
	healthHandler := health.NewHandler(health.NewService(db))
	authHandler   := auth.NewHandler(authSvc, usersSvc, auditSvc, limiter)
	usersHandler  := users.NewHandler(usersSvc, auditSvc, authSvc.HashPassword)
	setupHandler  := setup.NewHandler(setupSvc, usersSvc, authSvc.HashPassword, cfg.Auth.EncryptionKey)
	serverHandler := server.NewHandler(serverSvc)
	auditHandler  := audit.NewHandler(auditSvc)

	r := router.New(cfg, &router.Handlers{
		Health:   healthHandler,
		Auth:     authHandler,
		Users:    usersHandler,
		Setup:    setupHandler,
		Server:   serverHandler,
		Audit:    auditHandler,
		AuthSvc:  authSvc,
		SetupSvc: setupSvc,
		Limiter:  limiter,
	})

	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	log.Info().Str("addr", addr).Msg("server listening")
	if err := r.Run(addr); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}
