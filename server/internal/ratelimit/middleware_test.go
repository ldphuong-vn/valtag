package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ldphuong-vn/valtag/server/internal/agent"
)

// stubLimiter records the last key it was asked about and returns scripted
// results.
type stubLimiter struct {
	lastKey string
	calls   int
	allowed bool
	err     error
}

func (s *stubLimiter) Allow(_ context.Context, key string, _ int) (bool, error) {
	s.calls++
	s.lastKey = key
	return s.allowed, s.err
}

func handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// TestMiddleware_KeysOnAuthenticatedAgent: the limit key is the validated
// agent identity from the request context — a client-supplied X-Agent-ID
// header must be ignored (report §4.12: the old header keying let anyone
// rotate the header to evade the limit).
func TestMiddleware_KeysOnAuthenticatedAgent(t *testing.T) {
	stub := &stubLimiter{allowed: true}
	mw := Middleware(stub, 60)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(agent.WithAgentID(req.Context(), "agent-from-token"))
	req.Header.Set("X-Agent-ID", "spoofed-header-value")
	rec := httptest.NewRecorder()
	mw(handler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if stub.calls != 1 {
		t.Fatalf("limiter calls = %d, want 1", stub.calls)
	}
	if stub.lastKey != "agent-from-token" {
		t.Fatalf("limit key = %q, want the context agent identity", stub.lastKey)
	}
}

// TestMiddleware_HumanPassesThrough: no authenticated agent — limiter is
// not consulted (humans are covered by the IP-based limiter).
func TestMiddleware_HumanPassesThrough(t *testing.T) {
	stub := &stubLimiter{allowed: true}
	rec := httptest.NewRecorder()
	Middleware(stub, 60)(handler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if stub.calls != 0 {
		t.Fatalf("limiter consulted %d times for a human request, want 0", stub.calls)
	}
}

// TestMiddleware_RateLimited: exhausted agent gets 429.
func TestMiddleware_RateLimited(t *testing.T) {
	stub := &stubLimiter{allowed: false}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(agent.WithAgentID(req.Context(), "agent-1"))
	rec := httptest.NewRecorder()
	Middleware(stub, 60)(handler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", rec.Code)
	}
}

// TestMiddleware_FailClosedByDefault: a limiter backend error denies the
// request with 503 — a Redis outage must not open a bypass (report §4.12).
func TestMiddleware_FailClosedByDefault(t *testing.T) {
	stub := &stubLimiter{allowed: false, err: errors.New("redis down")}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(agent.WithAgentID(req.Context(), "agent-1"))
	rec := httptest.NewRecorder()
	Middleware(stub, 60)(handler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503 (fail-closed)", rec.Code)
	}
}

// TestMiddleware_FailOpenEscapeHatch: a limiter that opts to fail open
// (allowed=true with an error) lets the request through — the operator's
// availability-over-bypass-resistance choice.
func TestMiddleware_FailOpenEscapeHatch(t *testing.T) {
	stub := &stubLimiter{allowed: true, err: errors.New("redis down")}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(agent.WithAgentID(req.Context(), "agent-1"))
	rec := httptest.NewRecorder()
	Middleware(stub, 60)(handler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (fail-open)", rec.Code)
	}
}
