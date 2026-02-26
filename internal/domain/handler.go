package domain

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/audit"
	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

// Handler exposes domain management endpoints.
type Handler struct {
	svc      *Service
	auditSvc *audit.Service
}

func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, auditSvc: auditSvc}
}

// List godoc — GET /api/v1/domains
func (h *Handler) List(c *gin.Context) {
	domains, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch domains"))
		return
	}
	c.JSON(http.StatusOK, response.OK(domains))
}

// GetByID godoc — GET /api/v1/domains/:id
func (h *Handler) GetByID(c *gin.Context) {
	d, err := h.svc.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error("domain not found"))
		return
	}
	c.JSON(http.StatusOK, response.OK(d))
}

// Create godoc — POST /api/v1/domains
func (h *Handler) Create(c *gin.Context) {
	var req AddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	d, err := h.svc.Create(req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "domains.created",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "domain",
		ResourceID:   d.ID.String(),
		Details:      map[string]any{"name": d.Name},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusCreated, response.OK(d))
}

// Delete godoc — DELETE /api/v1/domains/:id
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	// Capture name before deletion for the audit log
	d, _ := h.svc.GetByID(id)

	if err := h.svc.Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	name := ""
	if d != nil {
		name = d.Name
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "domains.deleted",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "domain",
		ResourceID:   id,
		Details:      map[string]any{"name": name},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "domain deleted"}))
}
