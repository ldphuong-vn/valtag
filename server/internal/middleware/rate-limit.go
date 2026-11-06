package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/ldphuong-vn/valtag/server/internal/auth"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// RateLimiter implements an in-memory sliding window rate limiter.
type RateLimiter struct {
	maxRequests int
	window      time.Duration
	mu          sync.Mutex
	clients     map[string][]time.Time
}

// NewRateLimiter creates a RateLimiter and starts a background cleanup goroutine.
func NewRateLimiter(maxRequests int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		maxRequests: maxRequests,
		window:      window,
		clients:     make(map[string][]time.Time),
	}
	go rl.cleanup()
	return rl
}

// allow returns true if the key is within the rate limit window.
func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	timestamps := rl.clients[key]
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.maxRequests {
		rl.clients[key] = valid
		return false
	}

	rl.clients[key] = append(valid, now)
	return true
}

// cleanup removes expired entries every 5 minutes to prevent unbounded memory growth.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-rl.window)
		for key, timestamps := range rl.clients {
			valid := timestamps[:0]
			for _, t := range timestamps {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(rl.clients, key)
			} else {
				rl.clients[key] = valid
			}
		}
		rl.mu.Unlock()
	}
}

// IPMiddleware limits requests by remote IP address.
func (rl *RateLimiter) IPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(r.RemoteAddr) {
				// AX: sliding window — Retry-After = window length (conservative
				// upper bound; precise eviction time needs the oldest hit).
				apierror.RetryAfter(w, int(rl.window.Seconds()))
				apierror.TooManyRequests(w, "rate limit exceeded, try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Middleware limits requests by authenticated userID, falling back to remote IP.
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := auth.UserIDFromContext(r.Context())
			if key == "" {
				key = r.RemoteAddr
			}
			if !rl.allow(key) {
				apierror.RetryAfter(w, int(rl.window.Seconds()))
				apierror.TooManyRequests(w, "rate limit exceeded, try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
