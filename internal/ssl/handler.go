package ssl

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/audit"
	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

// Handler exposes SSL certificate management endpoints.
type Handler struct {
	svc      *Service
	auditSvc *audit.Service
}

func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, auditSvc: auditSvc}
}

// List godoc — GET /api/v1/ssl
func (h *Handler) List(c *gin.Context) {
	certs, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch SSL certs"))
		return
	}
	c.JSON(http.StatusOK, response.OK(certs))
}

// GetByDomainID godoc — GET /api/v1/ssl/domain/:domain_id
func (h *Handler) GetByDomainID(c *gin.Context) {
	cert, err := h.svc.GetByDomainID(c.Param("domain_id"))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error("no SSL cert found for this domain"))
		return
	}
	c.JSON(http.StatusOK, response.OK(cert))
}

// Provision godoc — POST /api/v1/ssl/domain/:domain_id/provision
// Requests a new Let's Encrypt certificate. Long-running — use async in production.
func (h *Handler) Provision(c *gin.Context) {
	domainID := c.Param("domain_id")
	var req ProvisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	cert, err := h.svc.Provision(domainID, req.IsWildcard)
	if err != nil {
		c.JSON(http.StatusBadGateway, response.Error("certificate provisioning failed: "+err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "ssl.provisioned",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "ssl_cert",
		ResourceID:   cert.ID.String(),
		Details:      map[string]any{"domain": cert.DomainName, "wildcard": cert.IsWildcard},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(cert))
}

// Renew godoc — POST /api/v1/ssl/:id/renew
func (h *Handler) Renew(c *gin.Context) {
	id := c.Param("id")
	cert, _ := h.svc.GetByID(id)
	if err := h.svc.Renew(id); err != nil {
		c.JSON(http.StatusBadGateway, response.Error("renewal failed: "+err.Error()))
		return
	}
	domain := ""
	if cert != nil {
		domain = cert.DomainName
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "ssl.renewed",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "ssl_cert",
		ResourceID:   id,
		Details:      map[string]any{"domain": domain},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "certificate renewed"}))
}

// ToggleRedirect godoc — PATCH /api/v1/ssl/:id/redirect
func (h *Handler) ToggleRedirect(c *gin.Context) {
	id := c.Param("id")
	var req RedirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if err := h.svc.ToggleRedirect(id, req.Enabled); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "ssl.redirect_toggled",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "ssl_cert",
		ResourceID:   id,
		Details:      map[string]any{"redirect_https": req.Enabled},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"redirect_https": req.Enabled}))
}
