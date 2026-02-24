package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error().
					Interface("panic", err).
					Str("path", c.Request.URL.Path).
					Msg("panic recovered")

				c.JSON(http.StatusInternalServerError, response.Error("internal server error"))
				c.Abort()
			}
		}()
		c.Next()
	}
}
