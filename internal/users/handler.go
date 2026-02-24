package users

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/audit"
	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

type Handler struct {
	svc      *Service
	auditSvc *audit.Service
	hashFn   func(string) (string, error)
}

func NewHandler(svc *Service, auditSvc *audit.Service, hashFn func(string) (string, error)) *Handler {
	return &Handler{svc: svc, auditSvc: auditSvc, hashFn: hashFn}
}

// List godoc — GET /api/v1/users
func (h *Handler) List(c *gin.Context) {
	users, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch users"))
		return
	}
	resp := make([]UserResponse, len(users))
	for i, u := range users {
		resp[i] = u.ToResponse()
	}
	c.JSON(http.StatusOK, response.OK(resp))
}

// Create godoc — POST /api/v1/users
func (h *Handler) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if req.Password != req.ConfirmPassword {
		c.JSON(http.StatusBadRequest, response.Error("passwords do not match"))
		return
	}
	if !req.Role.Valid() {
		c.JSON(http.StatusBadRequest, response.Error("invalid role"))
		return
	}
	hash, err := h.hashFn(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to hash password"))
		return
	}
	user := &User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         req.Role,
	}
	if err := h.svc.Create(user); err != nil {
		c.JSON(http.StatusConflict, response.Error("username or email already in use"))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "users.created",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "user",
		ResourceID:   user.ID.String(),
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusCreated, response.OK(user.ToResponse()))
}

// Update godoc — PUT /api/v1/users/:id
func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if err := h.svc.Update(id, &req, h.hashFn); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to update user"))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "users.updated",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "user",
		ResourceID:   id,
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "user updated"}))
}

// Delete godoc — DELETE /api/v1/users/:id
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	// Prevent deleting yourself
	if id == c.GetString("user_id") {
		c.JSON(http.StatusBadRequest, response.Error("cannot delete your own account"))
		return
	}
	if err := h.svc.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to delete user"))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "users.deleted",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "user",
		ResourceID:   id,
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "user deleted"}))
}
