package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func AuthMiddleware(log *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		userRole := c.GetHeader("X-User-Role")
		authMethod := c.GetHeader("X-Auth-Method")

		// Documentation is public, but we still capture the role if present for RBAC
		isDocsPath := c.FullPath() == "/api/v1/gamelift/docs" || c.FullPath() == "/api/v1/gamelift/docs/:slug"

		if userID == "" && !isDocsPath {
			log.Warnw("AUTH_MIDDLEWARE_MISSING_USER_ID",
				"remote_addr", c.Request.RemoteAddr,
				"path", c.FullPath(),
				"method", c.Request.Method,
				"headers", c.Request.Header,
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set("userID", userID)
		c.Set("userRole", userRole)
		c.Set("authMethod", authMethod)

		log.Infow("AUTH_MIDDLEWARE_OK",
			"user_id", userID,
			"role", userRole,
			"auth_method", authMethod,
			"path", c.FullPath(),
			"method", c.Request.Method,
			"remote_addr", c.Request.RemoteAddr,
		)

		c.Next()
	}
}