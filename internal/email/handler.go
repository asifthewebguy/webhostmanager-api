package email

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/audit"
	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

// Handler exposes email management endpoints.
type Handler struct {
	svc      *Service
	auditSvc *audit.Service
}

func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, auditSvc: auditSvc}
}

// List godoc — GET /api/v1/email
func (h *Handler) List(c *gin.Context) {
	accounts, err := h.svc.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch email accounts"))
		return
	}
	c.JSON(http.StatusOK, response.OK(accounts))
}

// MailServerStatus godoc — GET /api/v1/email/mail-server/status
func (h *Handler) MailServerStatus(c *gin.Context) {
	status, err := h.svc.CheckMailServerStatus()
	if err != nil {
		c.JSON(http.StatusBadGateway, response.Error("could not check mail server status: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.OK(status))
}

// InstallMailServer godoc — POST /api/v1/email/mail-server/install
func (h *Handler) InstallMailServer(c *gin.Context) {
	var req InstallMailServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if err := h.svc.InstallMailServer(req); err != nil {
		c.JSON(http.StatusConflict, response.Error(err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "email.mailserver.install_started",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "mail_server",
		Details:      map[string]any{"spam_assassin": req.SpamAssassin, "dkim": req.DKIM},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusAccepted, response.OK(gin.H{"message": "mail server installation started"}))
}

// GetInstallProgress godoc — GET /api/v1/email/mail-server/install/progress
func (h *Handler) GetInstallProgress(c *gin.Context) {
	progress := h.svc.GetInstallProgress()
	c.JSON(http.StatusOK, response.OK(progress))
}

// ListByDomain godoc — GET /api/v1/email/domain/:domain_id
func (h *Handler) ListByDomain(c *gin.Context) {
	accounts, err := h.svc.ListByDomain(c.Param("domain_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch email accounts"))
		return
	}
	c.JSON(http.StatusOK, response.OK(accounts))
}

// CreateAccount godoc — POST /api/v1/email/domain/:domain_id/accounts
func (h *Handler) CreateAccount(c *gin.Context) {
	domainID := c.Param("domain_id")
	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	account, err := h.svc.CreateAccount(domainID, req)
	if err != nil {
		c.JSON(http.StatusBadGateway, response.Error("failed to create email account: "+err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "email.account.created",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "email_account",
		ResourceID:   account.ID.String(),
		Details:      map[string]any{"email": account.Email, "domain": account.DomainName},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusCreated, response.OK(account))
}

// DeleteAccount godoc — DELETE /api/v1/email/accounts/:id
func (h *Handler) DeleteAccount(c *gin.Context) {
	account, err := h.svc.DeleteAccount(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "email.account.deleted",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "email_account",
		ResourceID:   account.ID.String(),
		Details:      map[string]any{"email": account.Email, "domain": account.DomainName},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "email account deleted"}))
}

// ChangePassword godoc — PATCH /api/v1/email/accounts/:id/password
func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if err := h.svc.ChangePassword(c.Param("id"), req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "email.account.password_changed",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "email_account",
		ResourceID:   c.Param("id"),
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "password updated"}))
}

// ChangeQuota godoc — PATCH /api/v1/email/accounts/:id/quota
func (h *Handler) ChangeQuota(c *gin.Context) {
	var req ChangeQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if err := h.svc.ChangeQuota(c.Param("id"), req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "quota updated", "quota_mb": req.QuotaMB}))
}

// ListForwarders godoc — GET /api/v1/email/domain/:domain_id/forwarders
func (h *Handler) ListForwarders(c *gin.Context) {
	fwds, err := h.svc.ListForwarders(c.Param("domain_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch forwarders"))
		return
	}
	c.JSON(http.StatusOK, response.OK(fwds))
}

// CreateForwarder godoc — POST /api/v1/email/domain/:domain_id/forwarders
func (h *Handler) CreateForwarder(c *gin.Context) {
	domainID := c.Param("domain_id")
	var req CreateForwarderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	fwd, err := h.svc.CreateForwarder(domainID, req)
	if err != nil {
		c.JSON(http.StatusBadGateway, response.Error("failed to create forwarder: "+err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "email.forwarder.created",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "email_forwarder",
		ResourceID:   fwd.ID.String(),
		Details:      map[string]any{"source": fwd.Source, "destination": fwd.Destination, "catch_all": fwd.IsCatchAll},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusCreated, response.OK(fwd))
}

// DeleteForwarder godoc — DELETE /api/v1/email/forwarders/:id
func (h *Handler) DeleteForwarder(c *gin.Context) {
	fwd, err := h.svc.DeleteForwarder(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "email.forwarder.deleted",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "email_forwarder",
		ResourceID:   fwd.ID.String(),
		Details:      map[string]any{"source": fwd.Source},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "forwarder deleted"}))
}

// GetConfig godoc — GET /api/v1/email/domain/:domain_id/config
func (h *Handler) GetConfig(c *gin.Context) {
	// Use the first account email for display; caller can pass ?email= param
	emailParam := c.Query("email")
	cfg, err := h.svc.GetConfig(emailParam)
	if err != nil {
		c.JSON(http.StatusBadGateway, response.Error("failed to get email config: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.OK(cfg))
}
