package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireOwner recusa requests de usuários com role != "owner".
// Deve ser usado após AuthMiddleware, que já popula ContextUserRole.
func RequireOwner(c *gin.Context) {
	role, _ := c.Get(ContextUserRole)
	if role != "owner" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "forbidden",
		})
		return
	}
	c.Next()
}
