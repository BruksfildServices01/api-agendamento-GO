package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxBodySize limita tamanho do body para evitar abuso/memória.
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = 1 << 20 // 1MB default
	}

	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
