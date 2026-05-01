package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Token-bucket in-memory por chave.
// Capacity = maxRequests, refill = maxRequests/windowSeconds por segundo.
// Em multi-instância cada réplica tem seu próprio bucket (use Redis para limites globais).
type bucket struct {
	tokens     float64
	lastRefill time.Time
}

type limiter struct {
	mu       sync.RWMutex
	buckets  map[string]*bucket
	capacity float64
	refillPS float64
	ttl      time.Duration

	// Cleanup assíncrono — evita O(n) no hot path de request.
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

func newLimiter(capacity int, refillPerSecond float64, ttl time.Duration) *limiter {
	l := &limiter{
		buckets:     make(map[string]*bucket),
		capacity:    float64(capacity),
		refillPS:    refillPerSecond,
		ttl:         ttl,
		stopCleanup: make(chan struct{}),
	}

	// Cleanup de buckets expirados a cada 5 minutos, fora do hot path.
	l.cleanupTicker = time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-l.cleanupTicker.C:
				l.cleanup()
			case <-l.stopCleanup:
				l.cleanupTicker.Stop()
				return
			}
		}
	}()

	return l
}

func (l *limiter) cleanup() {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.buckets {
		if now.Sub(b.lastRefill) > l.ttl {
			delete(l.buckets, k)
		}
	}
}

func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.capacity, lastRefill: now}
		l.buckets[key] = b
	}

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

// newInMemoryRateLimit cria um middleware de rate limit in-memory com semântica
// de maxRequests por windowSeconds.
func newInMemoryRateLimit(
	keyFn func(*gin.Context) string,
	maxRequests int,
	windowSeconds int,
) gin.HandlerFunc {
	refillPS := float64(maxRequests) / float64(windowSeconds)
	ttl := time.Duration(windowSeconds*3) * time.Second
	l := newLimiter(maxRequests, refillPS, ttl)

	return func(c *gin.Context) {
		now := time.Now()
		key := keyFn(c)

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

// RateLimitByKey mantido para compatibilidade interna.
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
	return newInMemoryRateLimit(keyFn, burst, 60/requestsPerMinute)
}
