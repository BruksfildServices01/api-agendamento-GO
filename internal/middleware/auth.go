package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

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

// Paths that bypass the subscription status check (billing and basic me info).
func skipSubscriptionCheck(path string) bool {
	return path == "/api/me" ||
		strings.HasPrefix(path, "/api/me/billing")
}

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

		// Fetch barbershop to verify existence and check subscription status.
		var shop struct {
			ID                    uint
			Status                string
			TrialEndsAt           *time.Time
			SubscriptionExpiresAt *time.Time
		}
		err = db.WithContext(c.Request.Context()).
			Table("barbershops").
			Select("id, status, trial_ends_at, subscription_expires_at").
			Where("id = ?", uint(barbershopID)).
			First(&shop).Error

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session_expired"})
				return
			}
			// On unexpected DB error, fail open to avoid blocking users.
			c.Set(ContextUserID, uint(userID))
			c.Set(ContextBarbershopID, uint(barbershopID))
			c.Set(ContextUserRole, role)
			c.Next()
			return
		}

		// Check subscription status (skip for billing and basic me endpoints).
		if !skipSubscriptionCheck(c.Request.URL.Path) {
			now := time.Now()
			blocked := false

			switch shop.Status {
			case "inactive", "suspended", "pending_payment":
				blocked = true
			case "trial":
				if shop.TrialEndsAt != nil && now.After(*shop.TrialEndsAt) {
					blocked = true
				}
			case "active":
				if shop.SubscriptionExpiresAt != nil && now.After(*shop.SubscriptionExpiresAt) {
					blocked = true
				}
			}

			if blocked {
				c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
					"error":  "subscription_required",
					"status": shop.Status,
				})
				return
			}
		}

		c.Set(ContextUserID, uint(userID))
		c.Set(ContextBarbershopID, uint(barbershopID))
		c.Set(ContextUserRole, role)

		c.Next()
	}
}
