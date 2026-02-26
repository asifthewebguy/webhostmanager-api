package dns

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/audit"
	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

// Handler exposes DNS management endpoints.
type Handler struct {
	svc      *Service
	auditSvc *audit.Service
}

func NewHandler(svc *Service, auditSvc *audit.Service) *Handler {
	return &Handler{svc: svc, auditSvc: auditSvc}
}

// ListByDomain godoc — GET /api/v1/dns/domain/:domain_id
func (h *Handler) ListByDomain(c *gin.Context) {
	records, err := h.svc.ListByDomain(c.Param("domain_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch DNS records"))
		return
	}
	c.JSON(http.StatusOK, response.OK(records))
}

// SyncFromCloudflare godoc — POST /api/v1/dns/domain/:domain_id/sync
func (h *Handler) SyncFromCloudflare(c *gin.Context) {
	domainID := c.Param("domain_id")
	records, err := h.svc.SyncFromCloudflare(domainID)
	if err != nil {
		c.JSON(http.StatusBadGateway, response.Error("cloudflare sync failed: "+err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "dns.synced",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "dns_zone",
		ResourceID:   domainID,
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(records))
}

// CreateRecord godoc — POST /api/v1/dns/domain/:domain_id/records
func (h *Handler) CreateRecord(c *gin.Context) {
	domainID := c.Param("domain_id")
	var req CreateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	record, err := h.svc.CreateRecord(domainID, req)
	if err != nil {
		c.JSON(http.StatusBadGateway, response.Error("failed to create DNS record: "+err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "dns.record.created",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "dns_record",
		ResourceID:   record.ID.String(),
		Details:      map[string]any{"type": record.Type, "name": record.Name, "content": record.Content},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusCreated, response.OK(record))
}

// UpdateRecord godoc — PUT /api/v1/dns/records/:id
func (h *Handler) UpdateRecord(c *gin.Context) {
	var req UpdateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	record, err := h.svc.UpdateRecord(c.Param("id"), req)
	if err != nil {
		c.JSON(http.StatusBadGateway, response.Error("failed to update DNS record: "+err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "dns.record.updated",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "dns_record",
		ResourceID:   record.ID.String(),
		Details:      map[string]any{"type": record.Type, "name": record.Name, "content": record.Content},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(record))
}

// DeleteRecord godoc — DELETE /api/v1/dns/records/:id
func (h *Handler) DeleteRecord(c *gin.Context) {
	record, err := h.svc.DeleteRecord(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	h.auditSvc.Write(audit.Entry{
		Action:       "dns.record.deleted",
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		ResourceType: "dns_record",
		ResourceID:   record.ID.String(),
		Details:      map[string]any{"type": record.Type, "name": record.Name},
		IPAddress:    c.ClientIP(),
	})
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "DNS record deleted"}))
}

// ToggleProxy godoc — PATCH /api/v1/dns/records/:id/proxy
func (h *Handler) ToggleProxy(c *gin.Context) {
	var req ToggleProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	record, err := h.svc.ToggleProxy(c.Param("id"), req)
	if err != nil {
		c.JSON(http.StatusBadGateway, response.Error("failed to toggle proxy: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.OK(record))
}
