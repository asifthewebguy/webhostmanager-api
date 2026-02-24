package health

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Check(c *gin.Context) {
	status := h.svc.Check()

	code := http.StatusOK
	if status.Database != "ok" {
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, response.OK(status))
}
