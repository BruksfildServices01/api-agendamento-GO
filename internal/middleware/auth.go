package middleware

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
)

// barbershopCacheEntry caches the subscription-status fields fetched per request.
// Entries expire after barbershopCacheTTL to keep billing state up-to-date.
const barbershopCacheTTL = 60 * time.Second

type barbershopCacheEntry struct {
	status                string
	trialEndsAt           *time.Time
	subscriptionExpiresAt *time.Time
	expiresAt             time.Time
}

var (
	barbershopCacheMu sync.Mutex
	barbershopCache   = make(map[uint]*barbershopCacheEntry)
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
		// Results are cached in-process for barbershopCacheTTL to avoid one DB
		// round-trip per authenticated request.
		bid := uint(barbershopID)

		var shopStatus string
		var shopTrialEndsAt *time.Time
		var shopSubscriptionExpiresAt *time.Time

		barbershopCacheMu.Lock()
		cached, hit := barbershopCache[bid]
		if hit && time.Now().Before(cached.expiresAt) {
			shopStatus = cached.status
			shopTrialEndsAt = cached.trialEndsAt
			shopSubscriptionExpiresAt = cached.subscriptionExpiresAt
			barbershopCacheMu.Unlock()
		} else {
			// Evict stale entry before releasing the lock so only one goroutine queries.
			delete(barbershopCache, bid)
			barbershopCacheMu.Unlock()

			var shop struct {
				ID                    uint
				Status                string
				TrialEndsAt           *time.Time
				SubscriptionExpiresAt *time.Time
			}
			err = db.WithContext(c.Request.Context()).
				Table("barbershops").
				Select("id, status, trial_ends_at, subscription_expires_at").
				Where("id = ?", bid).
				First(&shop).Error

			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session_expired"})
					return
				}
				// DB unavailable — fail closed to prevent unauthorized access.
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
				return
			}

			shopStatus = shop.Status
			shopTrialEndsAt = shop.TrialEndsAt
			shopSubscriptionExpiresAt = shop.SubscriptionExpiresAt

			barbershopCacheMu.Lock()
			barbershopCache[bid] = &barbershopCacheEntry{
				status:                shop.Status,
				trialEndsAt:           shop.TrialEndsAt,
				subscriptionExpiresAt: shop.SubscriptionExpiresAt,
				expiresAt:             time.Now().Add(barbershopCacheTTL),
			}
			barbershopCacheMu.Unlock()
		}

		// Check subscription status (skip for billing and basic me endpoints).
		if !skipSubscriptionCheck(c.Request.URL.Path) {
			now := time.Now()
			blocked := false

			switch shopStatus {
			case "inactive", "suspended", "pending_payment":
				blocked = true
			case "trial":
				if shopTrialEndsAt != nil && now.After(*shopTrialEndsAt) {
					blocked = true
				}
			case "active":
				if shopSubscriptionExpiresAt != nil && now.After(*shopSubscriptionExpiresAt) {
					blocked = true
				}
			}

			if blocked {
				c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
					"error":  "subscription_required",
					"status": shopStatus,
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
