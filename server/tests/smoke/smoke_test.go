//go:build smoke

// Package smoke runs the end-to-end happy path against a RUNNING valtag
// server (release gate, see .github/workflows/ci.yml job e2e-smoke):
//
//	health → register → login → create secret (auto-approve policy) →
//	request access → receive credential → read own audit trail
//
// It exercises the full HTTP stack — routing, auth middleware, versioned
// CLI tokens, policy resolution, envelope encryption at rest and lease-free
// credential delivery — the layers in-process integration tests cannot see.
package smoke

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

var baseURL = envOr("SMOKE_BASE_URL", "http://localhost:8080")

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// api is a minimal JSON API client that carries the bearer token.
type api struct {
	token string
}

func (c *api) call(t *testing.T, method, path string, body interface{}, wantStatus int) map[string]interface{} {
	t.Helper()
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body for %s %s: %v", method, path, err)
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, baseURL+path, payload)
	if err != nil {
		t.Fatalf("build request %s %s: %v", method, path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response %s %s: %v", method, path, err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s: got status %d, want %d — body: %s", method, path, resp.StatusCode, wantStatus, strings.TrimSpace(string(raw)))
	}
	out := map[string]interface{}{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			// Non-JSON success bodies (e.g. /health is plain) are fine.
			out["__raw"] = string(raw)
		}
	}
	return out
}

func str(m map[string]interface{}, keys ...string) string {
	var cur interface{} = m
	for _, k := range keys {
		obj, ok := cur.(map[string]interface{})
		if !ok {
			return ""
		}
		cur = obj[k]
	}
	s, _ := cur.(string)
	return s
}

// TestSmoke walks the ask-first happy path. Subtests share state and run in
// order; a failure stops the chain (t.FailNow in a helper keeps it short).
func TestSmoke(t *testing.T) {
	client := &api{}

	t.Run("01 server is healthy", func(t *testing.T) {
		client.call(t, http.MethodGet, "/health", nil, http.StatusOK)
	})

	email := fmt.Sprintf("smoke-%d@example.test", time.Now().UnixNano())

	t.Run("02 register a user", func(t *testing.T) {
		resp := client.call(t, http.MethodPost, "/api/v1/auth/register", map[string]string{
			"email": email, "password": "SmokePass123!", "region_code": "us",
		}, http.StatusCreated)
		client.token = str(resp, "access_token")
	})

	t.Run("03 login issues a bearer token", func(t *testing.T) {
		resp := client.call(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
			"email": email, "password": "SmokePass123!",
		}, http.StatusOK)
		token := str(resp, "access_token")
		if token == "" {
			t.Fatal("login returned no access_token")
		}
		client.token = token
	})

	var secretID string
	t.Run("04 create a secret with an auto-approve policy", func(t *testing.T) {
		resp := client.call(t, http.MethodPost, "/api/v1/secrets", map[string]string{
			"name":            "smoke-credential",
			"credential_type": "api_key",
			"value":           "sk-smoke-1234567890",
			"policy":          `{"allow_auto_approve":true,"require_approval":false,"require_reason":false,"max_duration":"1h"}`,
		}, http.StatusCreated)
		secretID = str(resp, "id")
		if secretID == "" {
			t.Fatalf("created secret has no id: %v", resp)
		}
	})

	var requestID string
	t.Run("05 request access is auto-approved", func(t *testing.T) {
		resp := client.call(t, http.MethodPost, "/api/v1/secrets/"+secretID+"/access-requests", map[string]interface{}{
			"reason":           "smoke: end-to-end ask-first flow",
			"duration_minutes": 30,
		}, http.StatusCreated)
		requestID = str(resp, "id")
		if status := str(resp, "status"); status != "approved" {
			t.Fatalf("request status = %q, want approved (auto-approve policy); body: %v", status, resp)
		}
		if requestID == "" {
			t.Fatal("approved request has no id")
		}
	})

	t.Run("06 credential is delivered for the approved request", func(t *testing.T) {
		resp := client.call(t, http.MethodGet, "/api/v1/credentials/"+requestID, nil, http.StatusOK)
		if got := str(resp, "value"); got != "sk-smoke-1234567890" {
			t.Fatalf("delivered credential = %q, want the secret plaintext", got)
		}
	})

	t.Run("07 own audit trail is readable", func(t *testing.T) {
		resp := client.call(t, http.MethodGet, "/api/v1/audit/logs", nil, http.StatusOK)
		logs, ok := resp["logs"].([]interface{})
		if !ok || len(logs) == 0 {
			t.Fatalf("audit logs empty for the acting user — scoping or logging broke: %v", resp)
		}
	})
}
