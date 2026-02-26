package notifications

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/audit"
	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

// Handler exposes notification management endpoints.
type Handler struct {
	svc      *Service
	auditSvc *audit.Service
}

func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, auditSvc: auditSvc}
}

// List godoc — GET /api/v1/notifications
func (h *Handler) List(c *gin.Context) {
	records, err := h.svc.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch notifications"))
		return
	}
	c.JSON(http.StatusOK, response.OK(records))
}

// ListUnread godoc — GET /api/v1/notifications/unread
func (h *Handler) ListUnread(c *gin.Context) {
	records, err := h.svc.ListUnread()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch notifications"))
		return
	}
	c.JSON(http.StatusOK, response.OK(records))
}

// UnreadCount godoc — GET /api/v1/notifications/unread-count
func (h *Handler) UnreadCount(c *gin.Context) {
	count, err := h.svc.UnreadCount()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch count"))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"count": count}))
}

// MarkRead godoc — PATCH /api/v1/notifications/:id/read
func (h *Handler) MarkRead(c *gin.Context) {
	if err := h.svc.MarkRead(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to mark notification as read"))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "marked as read"}))
}

// MarkAllRead godoc — PATCH /api/v1/notifications/read-all
func (h *Handler) MarkAllRead(c *gin.Context) {
	if err := h.svc.MarkAllRead(); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to mark all as read"))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "all notifications marked as read"}))
}

// GetChannelConfig godoc — GET /api/v1/notifications/config
func (h *Handler) GetChannelConfig(c *gin.Context) {
	cfg, err := h.svc.GetChannelConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch config"))
		return
	}
	c.JSON(http.StatusOK, response.OK(cfg))
}

// UpdateChannelConfig godoc — PUT /api/v1/notifications/config
func (h *Handler) UpdateChannelConfig(c *gin.Context) {
	var req UpdateChannelConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if err := h.svc.UpdateChannelConfig(req); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to update config"))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "notification.config.updated",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "notification_channel_config",
		ResourceID:   req.EventType + ":" + req.Channel,
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "config updated"}))
}

// TestChannel godoc — POST /api/v1/notifications/test/:channel
func (h *Handler) TestChannel(c *gin.Context) {
	channel := c.Param("channel")
	if err := h.svc.SendTestChannel(channel); err != nil {
		c.JSON(http.StatusBadGateway, response.Error("test failed: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "test notification sent to " + channel}))
}
