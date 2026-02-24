package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/users"
	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

// JWTMiddleware validates the Bearer token in Authorization header and
// stores user fields in the Gin context.
func JWTMiddleware(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.JSON(http.StatusUnauthorized, response.Error("missing or invalid authorization header"))
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := svc.ValidateToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, response.Error("invalid or expired token"))
			c.Abort()
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUsername, claims.Username)
		c.Set(CtxEmail, claims.Email)
		c.Set(CtxRole, claims.Role)
		c.Next()
	}
}

// RequireRole returns a middleware that allows only users with one of the
// specified roles. Must be placed after JWTMiddleware.
func RequireRole(roles ...users.Role) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[string(r)] = true
	}
	return func(c *gin.Context) {
		role := c.GetString(CtxRole)
		if !allowed[role] {
			c.JSON(http.StatusForbidden, response.Error("insufficient permissions"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// SetupNotCompleteMiddleware blocks requests to setup routes after setup is done.
func SetupNotCompleteMiddleware(isComplete func() bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isComplete() {
			c.JSON(http.StatusGone, response.Error("setup has already been completed"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// SetupRequiredMiddleware blocks dashboard routes until setup is complete.
func SetupRequiredMiddleware(isComplete func() bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isComplete() {
			c.JSON(http.StatusServiceUnavailable, response.Error("initial setup has not been completed"))
			c.Abort()
			return
		}
		c.Next()
	}
}
