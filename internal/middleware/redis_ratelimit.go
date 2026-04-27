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
// failOpen=true  → permite a requisição quando Redis está indisponível (padrão).
// failOpen=false → bloqueia a requisição quando Redis está indisponível (auth sensível).
type redisLimiter struct {
	client     *redis.Client
	maxReqs    int
	windowSecs int64
	failOpen   bool
}

func newRedisLimiter(redisURL string, maxReqs int, windowSecs int) (*redisLimiter, error) {
	return newRedisLimiterWithPolicy(redisURL, maxReqs, windowSecs, true)
}

func newRedisLimiterStrict(redisURL string, maxReqs int, windowSecs int) (*redisLimiter, error) {
	return newRedisLimiterWithPolicy(redisURL, maxReqs, windowSecs, false)
}

func newRedisLimiterWithPolicy(redisURL string, maxReqs int, windowSecs int, failOpen bool) (*redisLimiter, error) {
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
		failOpen:   failOpen,
	}, nil
}

func (r *redisLimiter) allow(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	windowID := time.Now().Unix() / r.windowSecs
	redisKey := fmt.Sprintf("rl:%s:%d", key, windowID)

	cnt, err := r.client.Incr(ctx, redisKey).Result()
	if err != nil {
		if r.failOpen {
			log.Printf("[RATELIMIT-REDIS] error: %v — fail-open, allowing request", err)
			return true
		}
		log.Printf("[RATELIMIT-REDIS] error: %v — fail-closed, blocking request", err)
		return false
	}

	if cnt == 1 {
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

// NewRateLimitByKeyStrict é idêntico a NewRateLimitByKey mas com fail-closed:
// se Redis estiver indisponível, a requisição é bloqueada com 503.
// Usar apenas em endpoints de autenticação sensível (login, registro, senha).
func NewRateLimitByKeyStrict(
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
		rl, err := newRedisLimiterStrict(redisURL, maxRequests, windowSeconds)
		if err != nil {
			log.Printf("[RATELIMIT-STRICT] Redis indisponível (%v) — bloqueando endpoint por segurança", err)
			// Se Redis nem conecta, bloqueia tudo neste endpoint
			return func(c *gin.Context) {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
					"error_code": "service_unavailable",
					"message":    "Serviço temporariamente indisponível.",
				})
			}
		}
		log.Printf("[RATELIMIT-STRICT] usando Redis fail-closed (%d req/%ds)", maxRequests, windowSeconds)
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

	// Sem Redis: in-memory é fail-safe por natureza (não tem estado distribuído)
	log.Printf("[RATELIMIT-STRICT] usando in-memory (%d req/%ds)", maxRequests, windowSeconds)
	return newInMemoryRateLimit(keyFn, maxRequests, windowSeconds)
}
