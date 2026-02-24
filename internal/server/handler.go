package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

// Handler exposes server metric and connection endpoints.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetMetrics godoc — GET /api/v1/server/metrics
func (h *Handler) GetMetrics(c *gin.Context) {
	m, err := h.svc.GetMetrics()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, response.Error("metrics unavailable: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.OK(m))
}

// GetInfo godoc — GET /api/v1/server/info
func (h *Handler) GetInfo(c *gin.Context) {
	info, err := h.svc.GetServerInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to collect server info"))
		return
	}
	c.JSON(http.StatusOK, response.OK(info))
}

// GetSummary godoc — GET /api/v1/server/summary
func (h *Handler) GetSummary(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(h.svc.GetSummary()))
}

// UpdateConnection godoc — POST /api/v1/server/connection
func (h *Handler) UpdateConnection(c *gin.Context) {
	var req ConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if err := h.svc.UpdateConfig(&req); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to save connection config"))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "connection config updated"}))
}

// TestConnection godoc — POST /api/v1/server/connection/test
func (h *Handler) TestConnection(c *gin.Context) {
	var req ConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if err := h.svc.TestConnection(&req); err != nil {
		c.JSON(http.StatusBadGateway, response.Error("connection test failed: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "connection successful"}))
}
