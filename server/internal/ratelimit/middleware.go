package ratelimit

import (
	"context"
	"net/http"

	"github.com/ldphuong-vn/valtag/server/internal/agent"
)

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
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":"rate limiter unavailable"}`))
				return
			}
			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Middleware rate-limits by authenticated agent identity (see Middleware).
func (l *RedisLimiter) Middleware(rpm int) func(http.Handler) http.Handler {
	return Middleware(l, rpm)
}
