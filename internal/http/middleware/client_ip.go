package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// Em proxys (Railway/Vercel), Gin costuma respeitar X-Forwarded-For se você configurar TrustedProxies.
// MVP: usamos c.ClientIP().
func ClientIPKey(c *gin.Context) string {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		ip = "unknown"
	}
	return ip
}
