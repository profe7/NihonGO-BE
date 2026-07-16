package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const contextUserIDKey = "userID"

func RequireAuth(tokens *TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed authorization header"})
			return
		}

		userID, err := tokens.Verify(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(contextUserIDKey, userID)
		c.Next()
	}
}

func UserIDFromContext(c *gin.Context) (int64, bool) {
	v, ok := c.Get(contextUserIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}
