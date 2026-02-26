package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/audit"
	"github.com/asifthewebguy/webhostmanager-api/internal/users"
	"github.com/asifthewebguy/webhostmanager-api/pkg/ratelimit"
	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

type Handler struct {
	authSvc  *Service
	usersSvc *users.Service
	auditSvc *audit.Service
	limiter  *ratelimit.LoginLimiter
}

func NewHandler(
	authSvc *Service,
	usersSvc *users.Service,
	auditSvc *audit.Service,
	limiter *ratelimit.LoginLimiter,
) *Handler {
	return &Handler{authSvc: authSvc, usersSvc: usersSvc, auditSvc: auditSvc, limiter: limiter}
}

// Login godoc
// POST /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error("username and password are required"))
		return
	}

	ip := c.ClientIP()
	user, err := h.usersSvc.FindByUsername(req.Username)
	if err != nil || !h.authSvc.VerifyPassword(user.PasswordHash, req.Password) {
		h.limiter.RecordFailure(ip)
		h.auditSvc.Write(audit.Entry{
			Action:    "auth.login.failed",
			Username:  req.Username,
			IPAddress: ip,
			UserAgent: c.Request.UserAgent(),
		})
		c.JSON(http.StatusUnauthorized, response.Error("invalid username or password"))
		return
	}

	if !user.IsActive {
		c.JSON(http.StatusForbidden, response.Error("account is disabled"))
		return
	}

	token, err := h.authSvc.GenerateToken(
		user.ID.String(), user.Username, user.Email, string(user.Role),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to generate token"))
		return
	}

	h.limiter.RecordSuccess(ip)
	h.usersSvc.UpdateLastLogin(user.ID)
	h.auditSvc.Write(audit.Entry{
		Action:    "auth.login.success",
		Username:  user.Username,
		Role:      string(user.Role),
		IPAddress: ip,
		UserAgent: c.Request.UserAgent(),
	})

	c.JSON(http.StatusOK, response.OK(LoginResponse{
		Token:    token,
		UserID:   user.ID.String(),
		Username: user.Username,
		Email:    user.Email,
		Role:     string(user.Role),
	}))
}

// Me godoc
// GET /api/v1/auth/me
func (h *Handler) Me(c *gin.Context) {
	userID := c.GetString(CtxUserID)
	user, err := h.usersSvc.FindByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error("user not found"))
		return
	}
	c.JSON(http.StatusOK, response.OK(user.ToResponse()))
}

// Logout godoc
// POST /api/v1/auth/logout  (stateless JWT — client must discard token)
func (h *Handler) Logout(c *gin.Context) {
	h.auditSvc.Write(audit.Entry{
		Action:    "auth.logout",
		Username:  c.GetString(CtxUsername),
		Role:      c.GetString(CtxRole),
		IPAddress: c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "logged out successfully"}))
}

// ChangePassword godoc
// PATCH /api/v1/auth/password — available to any authenticated user
func (h *Handler) ChangePassword(c *gin.Context) {
	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password"     binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	userID := c.GetString(CtxUserID)
	if err := h.usersSvc.ChangePassword(userID, req.CurrentPassword, req.NewPassword, h.authSvc.HashPassword); err != nil {
		if err.Error() == "current password is incorrect" {
			c.JSON(http.StatusUnauthorized, response.Error(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, response.Error("failed to change password"))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:    "auth.password.changed",
		Username:  c.GetString(CtxUsername),
		Role:      c.GetString(CtxRole),
		IPAddress: c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "password changed successfully"}))
}
