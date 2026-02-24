package audit

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

// Handler exposes audit log read endpoints.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// List godoc — GET /api/v1/audit-log?limit=N
func (h *Handler) List(c *gin.Context) {
	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	logs, err := h.svc.List(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("failed to fetch audit log"))
		return
	}
	c.JSON(http.StatusOK, response.OK(logs))
}
