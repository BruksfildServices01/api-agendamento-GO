package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set(
				"Access-Control-Allow-Headers",
				"Content-Type, Authorization",
			)
			c.Writer.Header().Set(
				"Access-Control-Allow-Methods",
				"GET, POST, PUT, PATCH, DELETE, OPTIONS",
			)
		}

		// 🔑 PRE-FLIGHT
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent) // 204
			return
		}

		c.Next()
	}
}
