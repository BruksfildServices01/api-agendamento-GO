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
// Algoritmo: janela fixa de 1 minuto; cada chave tem um contador INCR.
// Adequado para múltiplas réplicas — o limite é global, não por instância.
type redisLimiter struct {
	client            *redis.Client
	requestsPerMinute int
}

func newRedisLimiter(redisURL string, requestsPerMinute int) (*redisLimiter, error) {
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

	return &redisLimiter{client: client, requestsPerMinute: requestsPerMinute}, nil
}

func (r *redisLimiter) allow(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	redisKey := fmt.Sprintf("rl:%s:%d", key, time.Now().Minute())

	cnt, err := r.client.Incr(ctx, redisKey).Result()
	if err != nil {
		// Em caso de falha do Redis, permite a requisição (fail-open)
		log.Printf("[RATELIMIT-REDIS] error: %v — allowing request", err)
		return true
	}

	if cnt == 1 {
		// Primeira requisição neste minuto — define TTL de 2 minutos (janela + folga)
		r.client.Expire(ctx, redisKey, 2*time.Minute)
	}

	return int(cnt) <= r.requestsPerMinute
}

// NewRateLimitByKey cria o middleware de rate limit.
// Se redisURL estiver configurado, usa Redis (distribuído).
// Caso contrário, usa o bucket in-memory por instância.
func NewRateLimitByKey(
	keyFn func(*gin.Context) string,
	requestsPerMinute int,
	burst int,
	redisURL string,
) gin.HandlerFunc {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 60
	}
	if burst <= 0 {
		burst = 10
	}

	// Tenta Redis primeiro
	if redisURL != "" {
		rl, err := newRedisLimiter(redisURL, requestsPerMinute)
		if err != nil {
			log.Printf("[RATELIMIT] Redis indisponível (%v) — fallback para in-memory", err)
		} else {
			log.Println("[RATELIMIT] usando Redis distribuído")
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

	// Fallback: in-memory
	log.Println("[RATELIMIT] usando in-memory (não distribuído)")
	return RateLimitByKey(keyFn, requestsPerMinute, burst)
}
