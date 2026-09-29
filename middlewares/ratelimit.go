package middlewares

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const numShards = 64

type clientBucket struct {
	tokens     float64
	lastRefill time.Time
}

type limiterShard struct {
	mu          sync.Mutex
	clients     map[string]*clientBucket
	lastCleaned time.Time
}

type IPRateLimiter struct {
	shards       [numShards]*limiterShard
	rate         float64 // tokens per second
	burst        float64 // max bucket capacity
	cleanupAfter time.Duration
}

// hashIP computes a fast FNV-1a hash of the IP string to select a shard.
func hashIP(ip string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(ip); i++ {
		h ^= uint32(ip[i])
		h *= 16777619
	}
	return h % numShards
}

// NewIPRateLimiter creates an in-memory, sharded IP token-bucket rate limiter.
// Sharding across 64 mutexes eliminates lock contention under high concurrency (10,000+ RPS).
func NewIPRateLimiter(requestsPerMinute int, burst int) *IPRateLimiter {
	limiter := &IPRateLimiter{
		rate:         float64(requestsPerMinute) / 60.0,
		burst:        float64(burst),
		cleanupAfter: 5 * time.Minute,
	}

	now := time.Now()
	for i := 0; i < numShards; i++ {
		limiter.shards[i] = &limiterShard{
			clients:     make(map[string]*clientBucket),
			lastCleaned: now,
		}
	}

	return limiter
}

func (limiter *IPRateLimiter) allow(ip string) bool {
	shard := limiter.shards[hashIP(ip)]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	now := time.Now()

	// Periodic cleanup of stale client buckets within this shard
	if now.Sub(shard.lastCleaned) > limiter.cleanupAfter {
		for k, v := range shard.clients {
			if now.Sub(v.lastRefill) > limiter.cleanupAfter {
				delete(shard.clients, k)
			}
		}
		shard.lastCleaned = now
	}

	b, exists := shard.clients[ip]
	if !exists {
		shard.clients[ip] = &clientBucket{
			tokens:     limiter.burst - 1,
			lastRefill: now,
		}
		return true
	}

	// Calculate tokens added since last refill
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * limiter.rate
	if b.tokens > limiter.burst {
		b.tokens = limiter.burst
	}
	b.lastRefill = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}

	return false
}

// RateLimit returns a thread-safe in-memory IP-based rate limiter.
func RateLimit(requestsPerMinute int, burst int) gin.HandlerFunc {
	limiter := NewIPRateLimiter(requestsPerMinute, burst)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !limiter.allow(ip) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please slow down and try again later.",
				"code":  "RATE_LIMIT_EXCEEDED",
			})
			return
		}

		c.Next()
	}
}
