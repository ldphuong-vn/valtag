// Package discovery serves the agent-facing machine-readable description of
// this valtag deployment (AX — agent experience): what it is, how to
// authenticate, which contracts it speaks and where the docs live. Rendered
// from the running config so agents never guess.
package discovery

import (
	"encoding/json"
	"net/http"
)

// Doc is the /.well-known/valtag.json payload.
type Doc struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Repository  string   `json:"repository"`
	API         string   `json:"api"`
	Auth        Auth     `json:"authentication"`
	Endpoints   map[string]string `json:"endpoints"`
	MCP         MCP      `json:"mcp"`
	RateLimits  Limits   `json:"rate_limits"`
	Errors      Errors   `json:"errors"`
	Docs        []string `json:"docs"`
}

type Auth struct {
	API    string `json:"api"`
	Gateway string `json:"gateway"`
	GatewayContract string `json:"gateway_contract"`
}

type MCP struct {
	Transport string `json:"transport"`
	Binary    string `json:"binary"`
	Install   string `json:"install"`
}

type Limits struct {
	Headers []string `json:"headers"`
	Policy  string   `json:"policy"`
}

type Errors struct {
	Format          string `json:"format"`
	ProblemTypeBase string `json:"problem_type_base"`
}

// Handler returns the /.well-known/valtag.json handler. gatewayEnabled
// reflects GATEWAY_ENABLED so agents don't chase a listener that is off.
func Handler(gatewayEnabled bool, gatewayPort string) http.HandlerFunc {
	doc := Doc{
		Name:        "valtag",
		Description: "ask-first credential gate for AI agents: agents request, humans approve, leases expire, everything is audited.",
		Repository:  "https://github.com/ldphuong-vn/valtag",
		API:         "/api/v1",
		Auth: Auth{
			API:             "Bearer token (user JWT or agent token) on /api/v1",
			Gateway:         "Bearer valtag_dk_… derived key (lease-bound, short-lived)",
			GatewayContract: "POST /gateway/v1/authorize — 200 + real credential as headers, 401/403 fail-closed",
		},
		Endpoints: map[string]string{
			"health":             "/health",
			"metrics":            "/metrics (METRICS_ENABLED, internal only)",
			"gateway_authorize":  "/gateway/v1/authorize",
			"discovery":          "/.well-known/valtag.json",
		},
		MCP: MCP{
			Transport: "stdio",
			Binary:    "valtag-mcp-server",
			Install:   "valtag mcp install --ide claude|cursor",
		},
		RateLimits: Limits{
			Headers: []string{"RateLimit-Limit", "Retry-After"},
			Policy:  "per-agent per-minute fixed window; 429 carries Retry-After to the window boundary",
		},
		Errors: Errors{
			Format:          "RFC 9457 Problem Details (Accept: application/problem+json) + legacy envelope",
			ProblemTypeBase: "https://valtag.dev/problems/",
		},
		Docs: []string{
			"https://github.com/ldphuong-vn/valtag#readme",
			"/llms.txt",
		},
	}
	if !gatewayEnabled {
		doc.Endpoints["gateway_authorize"] = "disabled (GATEWAY_ENABLED=false)"
		doc.Auth.Gateway = "disabled on this deployment"
	}
	_ = gatewayPort // port is deployment-local; agents reach the gateway via its host, not this API
	b, _ := json.MarshalIndent(doc, "", "  ")
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(b)
	}
}

// LLMSText returns the /llms.txt handler — the llms.txt convention: a short,
// agent-oriented map of this service in plain markdown.
func LLMSText(gatewayEnabled bool) http.HandlerFunc {
	gatewaySection := ""
	if gatewayEnabled {
		gatewaySection = `
- Gateway: POST /gateway/v1/authorize with ` + "`Authorization: Bearer valtag_dk_…`" + ` (lease-bound key).
  200 returns the real upstream credential as headers (authorization + x-valtag-*); 401/403 = fail-closed.`
	}
	body := `# valtag

> Ask-first credential gate for AI agents: an agent requests access, a human
> approves, valtag mints a short-lived derived key (valtag_dk_…), the real
> upstream credential never leaves the vault, every action lands in a
> hash-chained audit trail.

- API: /api/v1 (Bearer user JWT or agent token)
- Health: /health; machine-readable discovery: /.well-known/valtag.json` + gatewaySection + `

- Errors: RFC 9457 Problem Details — send Accept: application/problem+json for
  the pure document; every error carries a stable code at either
  error.code (legacy envelope) or code (problem doc). Rate-limited responses
  carry RateLimit-Limit and Retry-After.
- MCP: valtag-mcp-server (stdio) — install with ` + "`valtag mcp install --ide claude`" + `.
- Source: https://github.com/ldphuong-vn/valtag
`
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write([]byte(body))
	}
}
