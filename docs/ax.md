# valtag AX — Agent Experience contract

> What an autonomous agent can rely on when talking to valtag. Humans get UX,
> developers get DX — this document is the third contract: the one agents read.
> Served live by the server at `/.well-known/valtag.json` and `/llms.txt`.

## 1. Discovery

| Endpoint | Content |
|---|---|
| `GET /.well-known/valtag.json` | machine-readable service description: auth schemes, endpoints, MCP, rate-limit policy, error format |
| `GET /llms.txt` | plain-markdown agent map of the service |

Both are cacheable (`Cache-Control: public, max-age=3600`) and reflect the
running config (e.g. `gateway_authorize` says `disabled` when
`GATEWAY_ENABLED=false`).

## 2. Authentication

| Surface | Scheme | Token |
|---|---|---|
| `/api/v1/**` | `Authorization: Bearer <token>` | user JWT or agent token |
| Gateway listener (`/gateway/v1/authorize`) | `Authorization: Bearer valtag_dk_…` | lease-bound derived key — **scheme prefix `Bearer` is required**; a bare key is ignored |

A derived key is short-lived by construction (lease TTL ≤ 1h) and dies on
revoke. Agents re-request after expiry; do not cache credentials beyond the
`x-valtag-lease-expires` header.

## 3. Errors — RFC 9457 Problem Details

Two wire formats, both always present:

- **Default envelope** (all endpoints): `{"error":{"code","message"}, "type",
  "title", "status", "detail", "instance"}` — legacy clients read
  `error.code`, agents read the Problem fields.
- **Pure Problem Details**: send `Accept: application/problem+json` →
  `application/problem+json` document with `type`, `title`, `status`,
  `detail`, `code` (stable machine code) and extension members.

`type` URIs are namespaced: `https://valtag.dev/problems/<code>` — e.g.
`…/problems/invalid_or_expired_credential`, `…/problems/rate_limit_exceeded`.

The gateway listener **always** speaks `application/problem+json` (it has no
legacy clients); status codes are contract: `200` allow, `401/403` fail-closed.

## 4. Rate limits

- Agent requests: per-agent per-minute fixed window. Headers on every
  agent-scoped response: `RateLimit-Limit`. On `429`:
  `Retry-After` = seconds to the window boundary.
- Human/IP requests: sliding window; `429` carries `Retry-After` = window length.
- Limiter backend down: `503` with `Retry-After: 5` (fail-closed default,
  `RATE_LIMIT_FAIL_OPEN=true` is the operator's escape hatch — report §4.12).

## 5. MCP

`valtag-mcp-server` (stdio) exposes the ask-first flow as tools:
`request_secret_access → check_approval_status → get_credential →
revoke_credential`, plus `request_dynamic_secret` (no-approval lease minting)
and local secret scanning. Tool descriptions embed the decision tree (what
each status means, what to call next) — an agent should never need this file
to use the flow.

## 6. CLI for scripts and agents

```bash
valtag request create <secret> --reason "…" --json   # → {"id","status","secret"}
valtag status <id> --json                            # poll until "approved"
valtag get <secret> --json                           # → {"secret","status","value"}
```

`valtag get` without `--json` prints the bare value on stdout (safe for
`$(valtag get NAME)` substitution); progress chatter goes to stderr in both
modes.

## 7. Backlog (declared, not yet built)

- `Idempotency-Key` on POST endpoints (agents retry; dedupe window needed)
- Outbound webhook when a request is approved (kill the polling loop)
- Signed agent identity (OIDC/mTLS) — M5 roadmap
