package transport

import (
	"log"
	"net/http"
	"strings"

	"backend/internal/domain"
	"backend/internal/interfaces"
	"backend/internal/transport/response"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware creates a Gin middleware that validates JWT tokens
// from either the Authorization header or the "token" query parameter.
func AuthMiddleware(authSvc interfaces.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string

		// 1. Check Authorization header
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// 2. Check query parameter
		if tokenStr == "" {
			tokenStr = c.Query("token")
		}

		if tokenStr == "" {
			response.SendError(c, http.StatusUnauthorized, "unauthorized-0")
			c.Abort()
			return
		}

		// Validate token
		claims, err := authSvc.ValidateToken(tokenStr)
		if err != nil {
			response.SendError(c, http.StatusUnauthorized, "unauthorized-1")
			log.Printf(err.Error())

			c.Abort()
			return
		}

		// Extract userID (prefer 'userId' claim, fallback to 'username')
		userID, ok := claims["userId"].(string)
		if !ok || userID == "" {
			userID, ok = claims["username"].(string)
		}

		if !ok || userID == "" {
			response.SendError(c, http.StatusUnauthorized, "invalid claims")
			c.Abort()
			return
		}

		// Inject userID into context
		c.Set(string(domain.UserIDKey), userID)
		c.Next()
	}
}
