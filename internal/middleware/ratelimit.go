package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Token-bucket simples por chave (in-memory).
// MVP: suficiente pra bloquear spam; em multi-replica, cada instância tem seu próprio bucket.
type bucket struct {
	tokens     float64
	lastRefill time.Time
}

type limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	capacity float64
	refillPS float64
	ttl      time.Duration
}

func newLimiter(capacity int, refillPerSecond float64, ttl time.Duration) *limiter {
	return &limiter{
		buckets:  make(map[string]*bucket),
		capacity: float64(capacity),
		refillPS: refillPerSecond,
		ttl:      ttl,
	}
}

func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// cleanup leve (MVP)
	for k, b := range l.buckets {
		if now.Sub(b.lastRefill) > l.ttl {
			delete(l.buckets, k)
		}
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.capacity, lastRefill: now}
		l.buckets[key] = b
	}

	// refill
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.refillPS
		if b.tokens > l.capacity {
			b.tokens = l.capacity
		}
		b.lastRefill = now
	}

	if b.tokens < 1 {
		return false
	}

	b.tokens -= 1
	return true
}

// RateLimitByKey cria um middleware de rate limit.
func RateLimitByKey(
	keyFn func(*gin.Context) string,
	requestsPerMinute int,
	burst int,
) gin.HandlerFunc {

	if requestsPerMinute <= 0 {
		requestsPerMinute = 60
	}
	if burst <= 0 {
		burst = 10
	}

	refillPS := float64(requestsPerMinute) / 60.0
	l := newLimiter(burst, refillPS, 10*time.Minute)

	return func(c *gin.Context) {
		now := time.Now()
		key := keyFn(c)

		// chave vazia -> bloqueia (defensivo)
		if strings.TrimSpace(key) == "" {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error_code": "rate_limited",
				"message":    "Muitas requisições. Tente novamente em instantes.",
			})
			return
		}

		if !l.allow(key, now) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error_code": "rate_limited",
				"message":    "Muitas requisições. Tente novamente em instantes.",
			})
			return
		}

		c.Next()
	}
}
