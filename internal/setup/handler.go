package setup

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/users"
	"github.com/asifthewebguy/webhostmanager-api/pkg/crypto"
	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

type Handler struct {
	svc      *Service
	usersSvc *users.Service
	hashFn   func(string) (string, error)
	encKey   string
}

func NewHandler(
	svc *Service,
	usersSvc *users.Service,
	hashFn func(string) (string, error),
	encKey string,
) *Handler {
	return &Handler{svc: svc, usersSvc: usersSvc, hashFn: hashFn, encKey: encKey}
}

// Status godoc — GET /api/v1/setup/status
func (h *Handler) Status(c *gin.Context) {
	status, err := h.svc.GetStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to get setup status"))
		return
	}
	c.JSON(http.StatusOK, response.OK(status))
}

// Step godoc — POST /api/v1/setup/step/:n
func (h *Handler) Step(c *gin.Context) {
	step := c.Param("n")
	switch step {
	case "1":
		h.step1(c)
	case "2":
		h.step2(c)
	case "3":
		h.step3(c)
	case "4":
		h.step4(c)
	case "5":
		h.step5(c)
	case "6":
		h.step6(c)
	case "7":
		h.step7(c)
	case "8":
		h.step8(c)
	default:
		c.JSON(http.StatusBadRequest, response.Error("invalid step number"))
	}
}

// step1 — acknowledge welcome screen.
func (h *Handler) step1(c *gin.Context) {
	if err := h.svc.AdvanceStep(1); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to advance step"))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{
		"step":    1,
		"message": "Welcome to WebHostManager",
		"version": "1.0.0",
	}))
}

// step2 — create Super Admin account.
func (h *Handler) step2(c *gin.Context) {
	var req Step2Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if req.Password != req.ConfirmPassword {
		c.JSON(http.StatusBadRequest, response.Error("passwords do not match"))
		return
	}

	// Only one super admin can be created via the wizard
	count, _ := h.usersSvc.CountSuperAdmins()
	if count > 0 {
		c.JSON(http.StatusConflict, response.Error("super admin account already exists"))
		return
	}

	hash, err := h.hashFn(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to hash password"))
		return
	}
	user := &users.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         users.RoleSuperAdmin,
	}
	if err := h.usersSvc.Create(user); err != nil {
		c.JSON(http.StatusConflict, response.Error("username or email already in use"))
		return
	}
	if err := h.svc.AdvanceStep(2); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to advance step"))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"step": 2, "message": "admin account created"}))
}

// step3 — configure server connection.
func (h *Handler) step3(c *gin.Context) {
	var req Step3Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	_ = h.svc.SaveSetting("server.connection_type", req.ConnectionType, false)
	if req.ConnectionType == "ssh" {
		_ = h.svc.SaveSetting("server.ssh_host", req.SSHHost, false)
		_ = h.svc.SaveSetting("server.ssh_port", fmt.Sprintf("%d", req.SSHPort), false)
		_ = h.svc.SaveSetting("server.ssh_user", req.SSHUser, false)
		_ = h.svc.SaveSetting("server.ssh_auth_type", req.SSHAuthType, false)
		if req.SSHKey != "" {
			if enc, err := crypto.Encrypt(req.SSHKey, h.encKey); err == nil {
				_ = h.svc.SaveSetting("server.ssh_key", enc, true)
			}
		}
		if req.SSHPassword != "" {
			if enc, err := crypto.Encrypt(req.SSHPassword, h.encKey); err == nil {
				_ = h.svc.SaveSetting("server.ssh_password", enc, true)
			}
		}
	}
	_ = h.svc.AdvanceStep(3)
	c.JSON(http.StatusOK, response.OK(gin.H{"step": 3, "message": "server connection configured"}))
}

// step4 — select reverse proxy.
func (h *Handler) step4(c *gin.Context) {
	var req Step4Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	_ = h.svc.SaveSetting("proxy.type", req.ProxyType, false)
	_ = h.svc.AdvanceStep(4)
	c.JSON(http.StatusOK, response.OK(gin.H{"step": 4, "message": "reverse proxy configured"}))
}

// step5 — optional domain setup.
func (h *Handler) step5(c *gin.Context) {
	var req Step5Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if !req.Skip && req.DefaultDomain != "" {
		_ = h.svc.SaveSetting("domain.default", req.DefaultDomain, false)
	}
	_ = h.svc.AdvanceStep(5)
	c.JSON(http.StatusOK, response.OK(gin.H{"step": 5, "message": "domain setup saved"}))
}

// step6 — optional Cloudflare API config.
func (h *Handler) step6(c *gin.Context) {
	var req Step6Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if !req.Skip {
		if enc, err := crypto.Encrypt(req.APIToken, h.encKey); err == nil {
			_ = h.svc.SaveSetting("cloudflare.api_token", enc, true)
		}
		_ = h.svc.SaveSetting("cloudflare.zone_id", req.ZoneID, false)
		_ = h.svc.SaveSetting("cloudflare.account_id", req.AccountID, false)
	}
	_ = h.svc.AdvanceStep(6)
	c.JSON(http.StatusOK, response.OK(gin.H{"step": 6, "message": "cloudflare configured"}))
}

// step7 — optional notification channel config.
func (h *Handler) step7(c *gin.Context) {
	var req Step7Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if !req.Skip {
		_ = h.svc.SaveSetting("smtp.host", req.SMTPHost, false)
		_ = h.svc.SaveSetting("smtp.port", fmt.Sprintf("%d", req.SMTPPort), false)
		_ = h.svc.SaveSetting("smtp.user", req.SMTPUser, false)
		_ = h.svc.SaveSetting("smtp.from", req.SMTPFrom, false)
		if req.SMTPPassword != "" {
			if enc, err := crypto.Encrypt(req.SMTPPassword, h.encKey); err == nil {
				_ = h.svc.SaveSetting("smtp.password", enc, true)
			}
		}
		if req.SlackWebhook != "" {
			if enc, err := crypto.Encrypt(req.SlackWebhook, h.encKey); err == nil {
				_ = h.svc.SaveSetting("notifications.slack_webhook", enc, true)
			}
		}
		if req.DiscordWebhook != "" {
			if enc, err := crypto.Encrypt(req.DiscordWebhook, h.encKey); err == nil {
				_ = h.svc.SaveSetting("notifications.discord_webhook", enc, true)
			}
		}
	}
	_ = h.svc.AdvanceStep(7)
	c.JSON(http.StatusOK, response.OK(gin.H{"step": 7, "message": "notifications configured"}))
}

// step8 — finalize setup.
func (h *Handler) step8(c *gin.Context) {
	if err := h.svc.Finalize(); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to finalize setup"))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"step": 8, "message": "setup complete — you can now log in"}))
}
