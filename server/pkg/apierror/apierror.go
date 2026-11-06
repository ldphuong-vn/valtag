package apierror

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error APIError `json:"error"`
	// RFC 9457 Problem Details fields — additive: existing clients keep
	// reading .error.code / .error.message while generic agents get a
	// parseable problem document without knowing valtag's envelope.
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Status   int    `json:"status,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// problemType returns the RFC 9457 "type" URI for a valtag error code.
func problemType(code string) string { return "https://valtag.dev/problems/" + code }

// Write emits the legacy envelope plus additive Problem Details fields.
// Kept as the default so every existing call site upgrades for free.
func Write(w http.ResponseWriter, statusCode int, code, message string) {
	writeEnvelope(w, statusCode, code, message, "", nil)
}

// WriteProblem is the RFC 9457-aware variant: when the client sends
// Accept: application/problem+json the pure Problem Details document is
// returned with Content-Type application/problem+json; otherwise it behaves
// exactly like Write. instance is r's path when r is non-nil; extra are
// extension members (e.g. retryable, lease_expires).
func WriteProblem(r *http.Request, w http.ResponseWriter, statusCode int, code, message string, extra map[string]any) {
	instance := ""
	if r != nil {
		instance = r.URL.Path
	}
	if r != nil && strings.Contains(r.Header.Get("Accept"), "application/problem+json") {
		writeProblemOnly(w, statusCode, code, message, instance, extra)
		return
	}
	writeEnvelope(w, statusCode, code, message, instance, extra)
}

func writeEnvelope(w http.ResponseWriter, statusCode int, code, message, instance string, extra map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	// Nothing left to do if the client has already gone or the response
	// fails to encode — the status code above is already sent.
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error:    APIError{Code: code, Message: message},
		Type:     problemType(code),
		Title:    http.StatusText(statusCode),
		Status:   statusCode,
		Detail:   message,
		Instance: instance,
	})
	// (extra members are intentionally only rendered on the problem+json path —
	// the legacy envelope stays stable apart from the additive fields above.)
}

func writeProblemOnly(w http.ResponseWriter, statusCode int, code, message, instance string, extra map[string]any) {
	doc := map[string]any{
		"type":   problemType(code),
		"title":  http.StatusText(statusCode),
		"status": statusCode,
		"detail": message,
		"code":   code, // extension member: stable valtag machine code
	}
	if instance != "" {
		doc["instance"] = instance
	}
	for k, v := range extra {
		doc[k] = v
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(doc)
}

// RetryAfter sets the standard Retry-After header (seconds).
func RetryAfter(w http.ResponseWriter, seconds int) {
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}

// Common helpers
func BadRequest(w http.ResponseWriter, message string) {
	Write(w, http.StatusBadRequest, "bad_request", message)
}

func Unauthorized(w http.ResponseWriter, message string) {
	Write(w, http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(w http.ResponseWriter, message string) {
	Write(w, http.StatusForbidden, "forbidden", message)
}

func NotFound(w http.ResponseWriter, message string) {
	Write(w, http.StatusNotFound, "not_found", message)
}

func Conflict(w http.ResponseWriter, message string) {
	Write(w, http.StatusConflict, "conflict", message)
}

func TooManyRequests(w http.ResponseWriter, message string) {
	Write(w, http.StatusTooManyRequests, "rate_limit_exceeded", message)
}

func InternalError(w http.ResponseWriter, message string) {
	Write(w, http.StatusInternalServerError, "internal_error", message)
}
