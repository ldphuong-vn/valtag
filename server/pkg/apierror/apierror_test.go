package apierror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func decode(t *testing.T, rec *httptest.ResponseRecorder) errorResponse {
	t.Helper()
	var r errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&r); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return r
}

func TestBadRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	BadRequest(rec, "test message")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status=%d", rec.Code)
	}
	r := decode(t, rec)
	if r.Error.Code != "bad_request" {
		t.Errorf("code=%q", r.Error.Code)
	}
	if r.Error.Message != "test message" {
		t.Errorf("msg=%q", r.Error.Message)
	}
}

func TestUnauthorized(t *testing.T) {
	rec := httptest.NewRecorder()
	Unauthorized(rec, "unauthorized")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d", rec.Code)
	}
	r := decode(t, rec)
	if r.Error.Code != "unauthorized" {
		t.Errorf("code=%q", r.Error.Code)
	}
}

func TestForbidden(t *testing.T) {
	rec := httptest.NewRecorder()
	Forbidden(rec, "forbidden")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status=%d", rec.Code)
	}
	r := decode(t, rec)
	if r.Error.Code != "forbidden" {
		t.Errorf("code=%q", r.Error.Code)
	}
}

func TestNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	NotFound(rec, "not found")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status=%d", rec.Code)
	}
	r := decode(t, rec)
	if r.Error.Code != "not_found" {
		t.Errorf("code=%q", r.Error.Code)
	}
}

func TestConflict(t *testing.T) {
	rec := httptest.NewRecorder()
	Conflict(rec, "conflict")
	if rec.Code != http.StatusConflict {
		t.Errorf("status=%d", rec.Code)
	}
	r := decode(t, rec)
	if r.Error.Code != "conflict" {
		t.Errorf("code=%q", r.Error.Code)
	}
}

func TestTooManyRequests(t *testing.T) {
	rec := httptest.NewRecorder()
	TooManyRequests(rec, "slow down")
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status=%d", rec.Code)
	}
	r := decode(t, rec)
	if r.Error.Code != "rate_limit_exceeded" {
		t.Errorf("code=%q", r.Error.Code)
	}
}

func TestInternalError(t *testing.T) {
	rec := httptest.NewRecorder()
	InternalError(rec, "oops")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status=%d", rec.Code)
	}
	r := decode(t, rec)
	if r.Error.Code != "internal_error" {
		t.Errorf("code=%q", r.Error.Code)
	}
}

func TestContentTypeHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	BadRequest(rec, "x")
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type=%q", ct)
	}
}

// AX: the default envelope also carries additive RFC 9457 fields so generic
// agents can parse any valtag error without knowing the legacy shape.
func TestProblemFieldsAdditive(t *testing.T) {
	rec := httptest.NewRecorder()
	BadRequest(rec, "boom")
	r := decode(t, rec)
	if r.Error.Code != "bad_request" || r.Error.Message != "boom" {
		t.Fatalf("legacy envelope broken: %+v", r.Error)
	}
	if r.Status != http.StatusBadRequest {
		t.Errorf("status field=%d", r.Status)
	}
	if r.Type != "https://valtag.dev/problems/bad_request" {
		t.Errorf("type=%q", r.Type)
	}
	if r.Title != "Bad Request" {
		t.Errorf("title=%q", r.Title)
	}
	if r.Detail != "boom" {
		t.Errorf("detail=%q", r.Detail)
	}
}

// AX: clients asking for application/problem+json get the pure RFC 9457 doc.
func TestWriteProblemNegotiation(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/thing", nil)
	r.Header.Set("Accept", "application/problem+json")
	rec := httptest.NewRecorder()
	WriteProblem(r, rec, http.StatusTooManyRequests, "rate_limit_exceeded", "slow down", map[string]any{"retry_after_seconds": 7})

	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content-type=%q", ct)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d", rec.Code)
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if doc["type"] != "https://valtag.dev/problems/rate_limit_exceeded" ||
		doc["title"] != "Too Many Requests" ||
		doc["status"] != float64(http.StatusTooManyRequests) ||
		doc["detail"] != "slow down" ||
		doc["code"] != "rate_limit_exceeded" ||
		doc["instance"] != "/api/v1/thing" ||
		doc["retry_after_seconds"] != float64(7) {
		t.Fatalf("problem doc mismatch: %v", doc)
	}
	// must NOT contain the legacy envelope
	if _, ok := doc["error"]; ok {
		t.Fatalf("problem+json must not contain legacy envelope")
	}
}

// Default path of WriteProblem (no Accept header) behaves like Write.
func TestWriteProblemDefaultEnvelope(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/thing", nil)
	rec := httptest.NewRecorder()
	WriteProblem(r, rec, http.StatusForbidden, "forbidden", "nope", nil)
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type=%q", ct)
	}
	d := decode(t, rec)
	if d.Error.Code != "forbidden" || d.Status != http.StatusForbidden || d.Instance != "/api/v1/thing" {
		t.Fatalf("envelope mismatch: %+v", d)
	}
}

func TestRetryAfter(t *testing.T) {
	rec := httptest.NewRecorder()
	RetryAfter(rec, 12)
	if got := rec.Header().Get("Retry-After"); got != "12" {
		t.Fatalf("retry-after=%q", got)
	}
}
