package gateway

import (
	"encoding/json"
	"net/http"
)

// writeProblem emits an RFC 9457 Problem Details document on the gateway
// listener. The listener has no legacy clients (every response body here is
// read by machines — Higress ext-auth reads the status, agents read the
// document), so it always speaks problem+json. Status codes are part of the
// authorize contract (200 allow / 401 403 fail-closed) and never change here.
func writeProblem(w http.ResponseWriter, statusCode int, code, detail string, extra map[string]any) {
	doc := map[string]any{
		"type":   "https://valtag.dev/problems/" + code,
		"title":  http.StatusText(statusCode),
		"status": statusCode,
		"detail": detail,
		"code":   code,
	}
	for k, v := range extra {
		doc[k] = v
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(doc)
}
