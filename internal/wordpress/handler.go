package wordpress

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/audit"
	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

// Handler exposes WordPress management endpoints.
type Handler struct {
	svc      *Service
	auditSvc *audit.Service
}

func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, auditSvc: auditSvc}
}

// List godoc — GET /api/v1/wordpress
func (h *Handler) List(c *gin.Context) {
	installs, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch WordPress installs"))
		return
	}
	c.JSON(http.StatusOK, response.OK(installs))
}

// GetByDomainID godoc — GET /api/v1/wordpress/domain/:domain_id
func (h *Handler) GetByDomainID(c *gin.Context) {
	install, err := h.svc.GetByDomainID(c.Param("domain_id"))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error("WordPress not installed on this domain"))
		return
	}
	c.JSON(http.StatusOK, response.OK(install))
}

// GetPlugins godoc — GET /api/v1/wordpress/domain/:domain_id/plugins
func (h *Handler) GetPlugins(c *gin.Context) {
	plugins, err := h.svc.GetPlugins(c.Param("domain_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.OK(plugins))
}

// Install godoc — POST /api/v1/wordpress/domain/:domain_id/install
func (h *Handler) Install(c *gin.Context) {
	domainID := c.Param("domain_id")
	var req InstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	install, err := h.svc.Install(domainID, req.AdminUser, req.AdminPass, req.AdminEmail)
	if err != nil {
		c.JSON(http.StatusBadGateway, response.Error("WordPress installation failed: "+err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "wordpress.installed",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "wordpress_install",
		ResourceID:   install.ID.String(),
		Details:      map[string]any{"domain": install.DomainName},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusCreated, response.OK(install))
}

// Uninstall godoc — DELETE /api/v1/wordpress/domain/:domain_id
func (h *Handler) Uninstall(c *gin.Context) {
	domainID := c.Param("domain_id")
	install, _ := h.svc.GetByDomainID(domainID)

	if err := h.svc.Uninstall(domainID); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}

	domain := ""
	if install != nil {
		domain = install.DomainName
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "wordpress.uninstalled",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "wordpress_install",
		Details:      map[string]any{"domain": domain},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "WordPress uninstalled"}))
}

// UpdateCore godoc — POST /api/v1/wordpress/domain/:domain_id/update-core
func (h *Handler) UpdateCore(c *gin.Context) {
	domainID := c.Param("domain_id")
	if err := h.svc.UpdateCore(domainID); err != nil {
		c.JSON(http.StatusBadGateway, response.Error("core update failed: "+err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "wordpress.core_updated",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "wordpress_install",
		Details:      map[string]any{"domain_id": domainID},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "WordPress core updated"}))
}

// UpdatePlugin godoc — POST /api/v1/wordpress/domain/:domain_id/plugins/:plugin/update
func (h *Handler) UpdatePlugin(c *gin.Context) {
	domainID := c.Param("domain_id")
	plugin := c.Param("plugin")
	if err := h.svc.UpdatePlugin(domainID, plugin); err != nil {
		c.JSON(http.StatusBadGateway, response.Error("plugin update failed: "+err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "wordpress.plugin_updated",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "wordpress_install",
		Details:      map[string]any{"domain_id": domainID, "plugin": plugin},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "plugin updated", "plugin": plugin}))
}

// ToggleDebug godoc — PATCH /api/v1/wordpress/domain/:domain_id/debug
func (h *Handler) ToggleDebug(c *gin.Context) {
	domainID := c.Param("domain_id")
	var req DebugRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if err := h.svc.ToggleDebug(domainID, req.Enable); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "wordpress.debug_toggled",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "wordpress_install",
		Details:      map[string]any{"domain_id": domainID, "debug": req.Enable},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"debug_mode": req.Enable}))
}
