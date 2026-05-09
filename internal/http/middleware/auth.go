package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"github.com/BruksfildServices01/barber-scheduler/internal/config"
)

// barbershopCacheTTL controla por quanto tempo o status da barbearia é mantido em
// memória. 30s é um equilíbrio razoável: reduz round-trips sem deixar uma janela
// longa para que uma assinatura expirada continue sendo aceita.
const barbershopCacheTTL = 30 * time.Second

type barbershopCacheEntry struct {
	status                string
	trialEndsAt           *time.Time
	subscriptionExpiresAt *time.Time
	expiresAt             time.Time
}

var (
	barbershopCacheMu sync.RWMutex
	barbershopCache   = make(map[uint]*barbershopCacheEntry)
	barbershopSFGroup singleflight.Group
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
		// Results are cached in-process for barbershopCacheTTL.
		// singleflight garante que requests concorrentes para o mesmo barbershop_id
		// disparam apenas uma query ao banco no cache miss (sem thundering herd).
		bid := uint(barbershopID)

		var shopStatus string
		var shopTrialEndsAt *time.Time
		var shopSubscriptionExpiresAt *time.Time

		barbershopCacheMu.RLock()
		cached, hit := barbershopCache[bid]
		validHit := hit && time.Now().Before(cached.expiresAt)
		barbershopCacheMu.RUnlock()

		if validHit {
			shopStatus = cached.status
			shopTrialEndsAt = cached.trialEndsAt
			shopSubscriptionExpiresAt = cached.subscriptionExpiresAt
		} else {
			type shopResult struct {
				status                string
				trialEndsAt           *time.Time
				subscriptionExpiresAt *time.Time
			}

			sfKey := fmt.Sprintf("barbershop:%d", bid)
			v, sfErr, _ := barbershopSFGroup.Do(sfKey, func() (any, error) {
				var shop struct {
					ID                    uint
					Status                string
					TrialEndsAt           *time.Time
					SubscriptionExpiresAt *time.Time
				}
				if err := db.WithContext(c.Request.Context()).
					Table("barbershops").
					Select("id, status, trial_ends_at, subscription_expires_at").
					Where("id = ?", bid).
					First(&shop).Error; err != nil {
					return nil, err
				}

				entry := &barbershopCacheEntry{
					status:                shop.Status,
					trialEndsAt:           shop.TrialEndsAt,
					subscriptionExpiresAt: shop.SubscriptionExpiresAt,
					expiresAt:             time.Now().Add(barbershopCacheTTL),
				}
				barbershopCacheMu.Lock()
				barbershopCache[bid] = entry
				barbershopCacheMu.Unlock()

				return shopResult{
					status:                shop.Status,
					trialEndsAt:           shop.TrialEndsAt,
					subscriptionExpiresAt: shop.SubscriptionExpiresAt,
				}, nil
			})

			if sfErr != nil {
				if errors.Is(sfErr, gorm.ErrRecordNotFound) {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session_expired"})
					return
				}
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
				return
			}

			res := v.(shopResult)
			shopStatus = res.status
			shopTrialEndsAt = res.trialEndsAt
			shopSubscriptionExpiresAt = res.subscriptionExpiresAt
		}

		// Cobrança de plataforma desativada — acesso livre para todos os usuários.
		_ = shopStatus
		_ = shopTrialEndsAt
		_ = shopSubscriptionExpiresAt

		c.Set(ContextUserID, uint(userID))
		c.Set(ContextBarbershopID, uint(barbershopID))
		c.Set(ContextUserRole, role)

		c.Next()
	}
}
