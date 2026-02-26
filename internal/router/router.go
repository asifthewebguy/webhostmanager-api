package router

import (
	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/audit"
	"github.com/asifthewebguy/webhostmanager-api/internal/auth"
	"github.com/asifthewebguy/webhostmanager-api/internal/config"
	"github.com/asifthewebguy/webhostmanager-api/internal/dns"
	"github.com/asifthewebguy/webhostmanager-api/internal/domain"
	"github.com/asifthewebguy/webhostmanager-api/internal/email"
	"github.com/asifthewebguy/webhostmanager-api/internal/health"
	"github.com/asifthewebguy/webhostmanager-api/internal/middleware"
	"github.com/asifthewebguy/webhostmanager-api/internal/notifications"
	"github.com/asifthewebguy/webhostmanager-api/internal/settings"
	"github.com/asifthewebguy/webhostmanager-api/internal/server"
	"github.com/asifthewebguy/webhostmanager-api/internal/setup"
	"github.com/asifthewebguy/webhostmanager-api/internal/ssl"
	"github.com/asifthewebguy/webhostmanager-api/internal/users"
	"github.com/asifthewebguy/webhostmanager-api/internal/wordpress"
	"github.com/asifthewebguy/webhostmanager-api/pkg/ratelimit"
)

// Handlers groups all feature handlers for injection into the router.
type Handlers struct {
	Health     *health.Handler
	Auth       *auth.Handler
	Users      *users.Handler
	Setup      *setup.Handler
	Server     *server.Handler
	Domain     *domain.Handler
	SSL        *ssl.Handler
	WordPress  *wordpress.Handler
	Email      *email.Handler
	DNS           *dns.Handler
	Notifications *notifications.Handler
	Settings      *settings.Handler
	Audit         *audit.Handler
	AuthSvc    *auth.Service
	SetupSvc   *setup.Service
	Limiter    *ratelimit.LoginLimiter
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

	// Auth — me/logout/password require JWT
	authProtected := v1.Group("/auth")
	authProtected.Use(auth.JWTMiddleware(h.AuthSvc))
	authProtected.GET("/me", h.Auth.Me)
	authProtected.POST("/logout", h.Auth.Logout)
	authProtected.PATCH("/password", h.Auth.ChangePassword)

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

	// Server metrics, connection & proxy
	serverGroup := protected.Group("/server")
	serverGroup.GET("/metrics", h.Server.GetMetrics)
	serverGroup.GET("/info", h.Server.GetInfo)
	serverGroup.GET("/summary", h.Server.GetSummary)
	serverGroup.POST("/connection", h.Server.UpdateConnection)
	serverGroup.POST("/connection/test", h.Server.TestConnection)
	serverGroup.GET("/proxy/status", h.Server.GetProxyStatus)
	serverGroup.POST("/proxy/restart", h.Server.RestartProxy)

	// Settings (super_admin only)
	settingsGroup := protected.Group("/settings")
	settingsGroup.Use(auth.RequireRole(users.RoleSuperAdmin))
	settingsGroup.GET("", h.Settings.List)
	settingsGroup.PUT("", h.Settings.Update)

	// Domain management
	domainsGroup := protected.Group("/domains")
	domainsGroup.GET("", h.Domain.List)
	domainsGroup.POST("", h.Domain.Create)
	domainsGroup.GET("/:id", h.Domain.GetByID)
	domainsGroup.DELETE("/:id", h.Domain.Delete)

	// SSL certificate management
	sslGroup := protected.Group("/ssl")
	sslGroup.GET("", h.SSL.List)
	sslGroup.GET("/domain/:domain_id", h.SSL.GetByDomainID)
	sslGroup.POST("/domain/:domain_id/provision", h.SSL.Provision)
	sslGroup.POST("/:id/renew", h.SSL.Renew)
	sslGroup.PATCH("/:id/redirect", h.SSL.ToggleRedirect)

	// WordPress management
	wpGroup := protected.Group("/wordpress")
	wpGroup.GET("", h.WordPress.List)
	wpGroup.GET("/domain/:domain_id", h.WordPress.GetByDomainID)
	wpGroup.GET("/domain/:domain_id/plugins", h.WordPress.GetPlugins)
	wpGroup.POST("/domain/:domain_id/install", h.WordPress.Install)
	wpGroup.DELETE("/domain/:domain_id", h.WordPress.Uninstall)
	wpGroup.POST("/domain/:domain_id/update-core", h.WordPress.UpdateCore)
	wpGroup.POST("/domain/:domain_id/plugins/:plugin/update", h.WordPress.UpdatePlugin)
	wpGroup.PATCH("/domain/:domain_id/debug", h.WordPress.ToggleDebug)

	// Email management
	emailGroup := protected.Group("/email")
	emailGroup.GET("", h.Email.List)
	emailGroup.GET("/mail-server/status", h.Email.MailServerStatus)
	emailGroup.POST("/mail-server/install", h.Email.InstallMailServer)
	emailGroup.GET("/mail-server/install/progress", h.Email.GetInstallProgress)
	emailGroup.GET("/domain/:domain_id", h.Email.ListByDomain)
	emailGroup.POST("/domain/:domain_id/accounts", h.Email.CreateAccount)
	emailGroup.DELETE("/accounts/:id", h.Email.DeleteAccount)
	emailGroup.PATCH("/accounts/:id/password", h.Email.ChangePassword)
	emailGroup.PATCH("/accounts/:id/quota", h.Email.ChangeQuota)
	emailGroup.GET("/domain/:domain_id/forwarders", h.Email.ListForwarders)
	emailGroup.POST("/domain/:domain_id/forwarders", h.Email.CreateForwarder)
	emailGroup.DELETE("/forwarders/:id", h.Email.DeleteForwarder)
	emailGroup.GET("/domain/:domain_id/config", h.Email.GetConfig)

	// DNS management
	dnsGroup := protected.Group("/dns")
	dnsGroup.GET("/domain/:domain_id", h.DNS.ListByDomain)
	dnsGroup.POST("/domain/:domain_id/sync", h.DNS.SyncFromCloudflare)
	dnsGroup.POST("/domain/:domain_id/records", h.DNS.CreateRecord)
	dnsGroup.PUT("/records/:id", h.DNS.UpdateRecord)
	dnsGroup.DELETE("/records/:id", h.DNS.DeleteRecord)
	dnsGroup.PATCH("/records/:id/proxy", h.DNS.ToggleProxy)

	// Notifications
	notifGroup := protected.Group("/notifications")
	notifGroup.GET("",                h.Notifications.List)
	notifGroup.GET("/unread",         h.Notifications.ListUnread)
	notifGroup.GET("/unread-count",   h.Notifications.UnreadCount)
	notifGroup.PATCH("/read-all",     h.Notifications.MarkAllRead)
	notifGroup.PATCH("/:id/read",     h.Notifications.MarkRead)
	notifGroup.GET("/config",         h.Notifications.GetChannelConfig)
	notifGroup.PUT("/config",         h.Notifications.UpdateChannelConfig)
	notifGroup.POST("/test/:channel", h.Notifications.TestChannel)

	// Audit log
	protected.GET("/audit-log", h.Audit.List)

	return r
}
