package router

import (
	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/audit"
	"github.com/asifthewebguy/webhostmanager-api/internal/auth"
	"github.com/asifthewebguy/webhostmanager-api/internal/config"
	"github.com/asifthewebguy/webhostmanager-api/internal/health"
	"github.com/asifthewebguy/webhostmanager-api/internal/middleware"
	"github.com/asifthewebguy/webhostmanager-api/internal/server"
	"github.com/asifthewebguy/webhostmanager-api/internal/setup"
	"github.com/asifthewebguy/webhostmanager-api/internal/users"
	"github.com/asifthewebguy/webhostmanager-api/pkg/ratelimit"
)

// Handlers groups all feature handlers for injection into the router.
type Handlers struct {
	Health   *health.Handler
	Auth     *auth.Handler
	Users    *users.Handler
	Setup    *setup.Handler
	Server   *server.Handler
	Audit    *audit.Handler
	AuthSvc  *auth.Service
	SetupSvc *setup.Service
	Limiter  *ratelimit.LoginLimiter
}

func New(cfg *config.Config, h *Handlers) *gin.Engine {
	if cfg.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS(cfg.App.AllowedOrigins))

	v1 := r.Group("/api/v1")

	// Public
	v1.GET("/health", h.Health.Check)

	// Setup wizard — public; steps blocked after completion
	setupGroup := v1.Group("/setup")
	setupGroup.GET("/status", h.Setup.Status)
	setupGroup.POST("/step/:n",
		auth.SetupNotCompleteMiddleware(h.SetupSvc.IsComplete),
		h.Setup.Step,
	)

	// Auth — login is public (rate-limited)
	authGroup := v1.Group("/auth")
	authGroup.POST("/login", h.Limiter.Middleware(), h.Auth.Login)

	// Auth — me/logout require JWT
	authProtected := v1.Group("/auth")
	authProtected.Use(auth.JWTMiddleware(h.AuthSvc))
	authProtected.GET("/me", h.Auth.Me)
	authProtected.POST("/logout", h.Auth.Logout)

	// Protected — requires setup complete + valid JWT
	protected := v1.Group("")
	protected.Use(auth.SetupRequiredMiddleware(h.SetupSvc.IsComplete))
	protected.Use(auth.JWTMiddleware(h.AuthSvc))

	// User management — Super Admin only
	usersGroup := protected.Group("/users")
	usersGroup.Use(auth.RequireRole(users.RoleSuperAdmin))
	usersGroup.GET("", h.Users.List)
	usersGroup.POST("", h.Users.Create)
	usersGroup.PUT("/:id", h.Users.Update)
	usersGroup.DELETE("/:id", h.Users.Delete)

	// Server metrics & connection
	serverGroup := protected.Group("/server")
	serverGroup.GET("/metrics", h.Server.GetMetrics)
	serverGroup.GET("/info", h.Server.GetInfo)
	serverGroup.GET("/summary", h.Server.GetSummary)
	serverGroup.POST("/connection", h.Server.UpdateConnection)
	serverGroup.POST("/connection/test", h.Server.TestConnection)

	// Audit log
	protected.GET("/audit-log", h.Audit.List)

	return r
}
