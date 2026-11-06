package ratelimit

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/ldphuong-vn/valtag/server/internal/agent"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// secondsToNextMinute: the agent limiter is a per-minute fixed window, so a
// denied request retries at the next boundary (Retry-After, RFC headers).
func secondsToNextMinute(now time.Time) int {
	return 60 - now.Second()
}

// Limiter is the minimal contract the middleware needs — RedisLimiter
// implements it; tests can stub it.
type Limiter interface {
	Allow(ctx context.Context, key string, rpm int) (bool, error)
}

// Middleware returns an HTTP middleware that rate-limits per authenticated
// agent. The key comes from the request context (set by the dual-auth
// middleware after token validation) — NOT from the client-supplied
// X-Agent-ID header, which any caller could rotate to evade the limit
// (report §4.12). Requests without an authenticated agent (human users)
// pass through; they are covered by the IP-based limiter.
//
// Error semantics: when the limiter backend fails, the decision follows the
// limiter's failOpen setting — deny with 503 (fail-closed, the secure
// default) or pass through (fail-open escape hatch).
func Middleware(l Limiter, rpm int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			agentID := agent.AgentIDFromContext(r.Context())
			if agentID == "" {
				next.ServeHTTP(w, r)
				return
			}
			allowed, err := l.Allow(r.Context(), agentID, rpm)
			if err != nil {
				if allowed { // limiter opted to fail open
					next.ServeHTTP(w, r)
					return
				}
				// AX: fail-closed 503 with standard retry guidance (report §4.12).
				w.Header().Set("RateLimit-Limit", strconv.Itoa(rpm))
				apierror.RetryAfter(w, 5)
				apierror.Write(w, http.StatusServiceUnavailable,
					"rate_limiter_unavailable", "rate limiter unavailable")
				return
			}
			if !allowed {
				// AX: standard rate-limit headers + retry at the window boundary.
				w.Header().Set("RateLimit-Limit", strconv.Itoa(rpm))
				apierror.RetryAfter(w, secondsToNextMinute(time.Now()))
				apierror.TooManyRequests(w, "rate limit exceeded")
				return
			}
			w.Header().Set("RateLimit-Limit", strconv.Itoa(rpm))
			next.ServeHTTP(w, r)
		})
	}
}

// Middleware rate-limits by authenticated agent identity (see Middleware).
func (l *RedisLimiter) Middleware(rpm int) func(http.Handler) http.Handler {
	return Middleware(l, rpm)
}
