package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
)

const (
	ContextUserID       = "userID"
	ContextBarbershopID = "barbershopID"
	ContextUserRole     = "userRole"
)

func AuthMiddleware(cfg *config.Config, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing_authorization_header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_authorization_header"})
			return
		}

		tokenString := parts[1]

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenMalformed
			}
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_token_claims"})
			return
		}

		userID, ok1 := claims["sub"].(float64)
		barbershopID, ok2 := claims["barbershopId"].(float64)
		role, _ := claims["role"].(string)
		if !ok1 || !ok2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_token_payload"})
			return
		}

		// Verify the barbershop still exists in DB (handles stale tokens after DB reset).
		var exists int64
		db.WithContext(c.Request.Context()).
			Table("barbershops").
			Where("id = ?", uint(barbershopID)).
			Count(&exists)
		if exists == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session_expired"})
			return
		}

		c.Set(ContextUserID, uint(userID))
		c.Set(ContextBarbershopID, uint(barbershopID))
		c.Set(ContextUserRole, role)

		c.Next()
	}
}
