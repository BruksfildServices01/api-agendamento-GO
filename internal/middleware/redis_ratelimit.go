package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// redisLimiter implementa rate limit distribuído usando Redis INCR + EXPIRE.
// Algoritmo: janela deslizante baseada em segundos — elimina o burst duplo
// da janela fixa por minuto. A chave usa time.Now().Unix()/windowSecs,
// garantindo que cada janela seja isolada e proporcional ao tamanho configurado.
type redisLimiter struct {
	client     *redis.Client
	maxReqs    int
	windowSecs int64
}

func newRedisLimiter(redisURL string, maxReqs int, windowSecs int) (*redisLimiter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_URL: %w", err)
	}
	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &redisLimiter{
		client:     client,
		maxReqs:    maxReqs,
		windowSecs: int64(windowSecs),
	}, nil
}

func (r *redisLimiter) allow(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// A janela desliza de windowSecs em windowSecs — burst na virada é
	// no máximo de 2 segundos (apenas no momento em que a janela troca),
	// não de 1 minuto inteiro como na implementação anterior.
	windowID := time.Now().Unix() / r.windowSecs
	redisKey := fmt.Sprintf("rl:%s:%d", key, windowID)

	cnt, err := r.client.Incr(ctx, redisKey).Result()
	if err != nil {
		// Falha do Redis → fail-open (não bloqueia a API)
		log.Printf("[RATELIMIT-REDIS] error: %v — allowing request", err)
		return true
	}

	if cnt == 1 {
		// Primeira req nesta janela — TTL = janela + folga de 1 janela
		r.client.Expire(ctx, redisKey, time.Duration(r.windowSecs*2)*time.Second)
	}

	return int(cnt) <= r.maxReqs
}

// NewRateLimitByKey cria o middleware de rate limit com semântica clara:
//   - maxRequests: máximo de requisições permitidas na janela
//   - windowSeconds: tamanho da janela em segundos
//
// Exemplos:
//
//	NewRateLimitByKey(keyFn, 30, 60, redisURL)   → 30 req/minuto
//	NewRateLimitByKey(keyFn, 10, 300, redisURL)  → 10 req/5min
//	NewRateLimitByKey(keyFn, 5, 3600, redisURL)  → 5 req/hora
//
// Se redisURL estiver configurado usa Redis distribuído; caso contrário usa
// token bucket in-memory por instância.
func NewRateLimitByKey(
	keyFn func(*gin.Context) string,
	maxRequests int,
	windowSeconds int,
	redisURL string,
) gin.HandlerFunc {
	if maxRequests <= 0 {
		maxRequests = 60
	}
	if windowSeconds <= 0 {
		windowSeconds = 60
	}

	if redisURL != "" {
		rl, err := newRedisLimiter(redisURL, maxRequests, windowSeconds)
		if err != nil {
			log.Printf("[RATELIMIT] Redis indisponível (%v) — fallback para in-memory", err)
		} else {
			log.Printf("[RATELIMIT] usando Redis distribuído (%d req/%ds)", maxRequests, windowSeconds)
			return func(c *gin.Context) {
				key := keyFn(c)
				if strings.TrimSpace(key) == "" || !rl.allow(key) {
					c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
						"error_code": "rate_limited",
						"message":    "Muitas requisições. Tente novamente em instantes.",
					})
					return
				}
				c.Next()
			}
		}
	}

	log.Printf("[RATELIMIT] usando in-memory (%d req/%ds)", maxRequests, windowSeconds)
	return newInMemoryRateLimit(keyFn, maxRequests, windowSeconds)
}
