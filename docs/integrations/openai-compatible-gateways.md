# valtag × OpenAI-compatible proxies — the inbound-key recipe

Self-hosted LLM proxies (CLIProxyAPI, new-api, 9Router, OmniRoute, …) all
expose an OpenAI-compatible API and authenticate callers with their own
static key. valtag does not run inside them — an edge proxy with
forward-auth does the swap:

```text
Agent ── Authorization: Bearer valtag_dk_… ──▶ EDGE (nginx/Caddy/Traefik)
                                               │ 1. authorize against valtag (fail-closed)
                                               │ 2. REPLACE Authorization with the proxy's own key
                                               │    (the key is a valtag provider master_key — it never
                                               │     lives in agent configs)
                                               │ 3. STRIP client-sent x-valtag-* and every alternative key carrier
                                               ▼
                                    LLM proxy (:8317/:3001/…) sees its own key
```

**Setup in valtag:** create a `derived_api_key` provider whose `master_key`
is the proxy's inbound key (`POST /api/v1/projects/{id}/providers`). An
approved lease returns `valtag_dk_…` to the agent; valtag's authorize
endpoint swaps it back to the master_key on the `Authorization` response
header (verified — see the matrix below). Give **one provider per proxy**
if you want per-proxy attribution; give one per agent (per-agent inbound
keys) if the proxy supports multiple keys and you want per-agent
attribution.

**Status of this page:**

| Proxy | Verified | What was run |
|---|---|---|
| CLIProxyAPI (58.9k⭐) | ✅ live E2E | auth matrix, fail-open trap, edge swap, header-strip test (staging 2026-11-18) |
| new-api (52.2k⭐) | ✅ live E2E (auth layer) | auth matrix, carrier traps, dash-split trap, edge swap + strips (staging 2026-11-18) |
| 9Router | 🟡 from source reading | not run yet |
| OmniRoute | 🟡 from source reading | not run yet |

✅ = we ran it end-to-end (valtag gate → nginx edge → proxy). 🟡 = behavior
read from the project's source code, not run — treat as a starting point.

## The edge (nginx, verified config)

```nginx
server {
  listen 8319;
  location / {
    if ($arg_key) { return 401; }   # kill the ?key= carrier (new-api / Gemini routes)
    auth_request /_valtag_auth;
    auth_request_set $valtag_credential $upstream_http_authorization;
    proxy_set_header Authorization $valtag_credential;   # REPLACE with the proxy's key
    proxy_set_header x-api-key "";
    proxy_set_header x-goog-api-key "";
    proxy_set_header mj-api-secret "";
    proxy_set_header x-valtag-agent-id "";               # strip client-sent valtag headers
    proxy_set_header x-valtag-lease-id "";
    proxy_set_header x-valtag-lease-expires "";
    proxy_pass http://127.0.0.1:8317;
  }
  location = /_valtag_auth {
    internal;
    proxy_pass http://127.0.0.1:10256/gateway/v1/authorize;
    proxy_pass_request_body off;
    proxy_set_header Content-Length "";
    proxy_set_header Authorization $http_authorization;
  }
}
```

Verified with an echo upstream: a client sending fake
`x-valtag-agent-id: attacker-fake` through the edge produces **0
occurrences** of the fake value and **0 occurrences** of `valtag_dk_` at the
upstream; the upstream sees only the swapped credential.

**The strip/replace rule applies to every gateway.** Downstream must never
trust `x-valtag-*` sent by the client (precedent: Higress
GHSA-m4wc-gcm6-7m94 — an identity header that was appended instead of
replaced), and must never see the derived key.

## Per-proxy traps (each one verified or source-read, as labeled)

### CLIProxyAPI — ✅ verified live

- **Fail-open when no access provider is configured.** With an empty
  `access:` section, `GET /v1/models` returns **200 without any key**, and
  chat requests go straight into provider selection (proven live). Configure
  at least one entry in `access.api-keys` and never expose CPA without one.
- `/v1/tunnel` checks its own key (401 without one — proven live); the
  management API (`/v0/management/…`) uses a separate management key.
  Bind the proxy to loopback/private networking and keep both surfaces off
  the public edge.
- One static inbound key per proxy in `access.api-keys`; use the valtag
  provider's `master_key` value as that entry.

### new-api — ✅ verified live (auth layer)

- **`x-api-key` beats `Authorization`** — a request with a valid
  `Authorization` and a bogus `x-api-key` is **401** (proven live). The edge
  must clear `x-api-key` or the swap is broken.
- **`?key=` is a live carrier** — `GET /v1/models?key=<valid>` with no
  header returns 200 (proven live; the Gemini-compatible route also reads
  `x-goog-api-key`). The edge must deny `?key=` requests (or forward them
  pre-authenticated deliberately).
- **`mj-api-secret` is a carrier too** (midjourney routes).
- **The key is compared only up to the first dash.** `middleware/auth.go`
  (`TokenAuth`) does `strings.TrimPrefix(key, "sk-")` →
  `strings.Split(key, "-")` → `parts[0]` before validating. Consequences:
  server-generated token keys (48 alphanumerics, no dash) are fine, but a
  valtag credential **containing a dash is silently truncated** and the
  request 401s. When you put a new-api key into a valtag provider's
  `master_key`, use a dash-free value (verified live: a credential with
  dashes matched only its first segment and failed; the dash-free prefix
  matched).
- The test stack had no channels configured, so a *valid* credential yields
  `400 Invalid channel ID` — that is the auth layer passing; the 400 is the
  model-routing layer, not the gate.
- The web UI (`:3000`/setup) and the admin API are separate surfaces — keep
  them off the public edge, and mind `TRUSTED_PROXIES`/token `allow_ips` for
  direct-access defense (from docs).

### 9Router — 🟡 from source reading, not run

- Requests from **loopback without proxy headers are treated as "local" and
  skip auth** (`custom-server.js`) — when the edge runs on the same host,
  make sure the forwarded request carries proxy headers (e.g.
  `X-Forwarded-For`) so it is not treated as local, and block direct access.
- Multiple key carriers (`x-api-key`, `x-goog-api-key`, `?key=`) plus
  per-key model allow-lists (unknown keys = unrestricted). Strip carriers at
  the edge; store one 9Router key per agent if you want attribution.

### OmniRoute — 🟡 from source reading, not run

- **`REQUIRE_API_KEY=false` by default** and an invalid bearer **degrades to
  anonymous** (`policies/clientApi.ts`) — the edge alone does not make this
  safe; set `REQUIRE_API_KEY=true` first.
- Cover `/a2a` and `/api/mcp` (same keys), and the `/vscode/<key>` path
  carrier, in the edge config.

## Compliance framing

This recipe is for proxies you **self-host with credentials you are entitled
to use** (API-key providers, your own subscriptions used within the
provider's terms). Several of these projects advertise pooling consumer
subscription OAuth accounts — that behavior is unrelated to valtag and
outside its terms-safe path; valtag's role here is only the inbound gate for
your own keys. Proxy-specific terms still apply.

## Verify checklist (what "✅" above means)

Per proxy, on your own stack: with the edge in front and the proxy's key
stored as a valtag provider `master_key` —

1. valid lease → `200` at the edge, upstream sees the swapped key, never
   `valtag_dk_`;
2. no key → `401` at the edge (fail-closed);
3. dead/revoked lease → `401`;
4. valtag down → deny (nginx `auth_request`: 500/503, never open);
5. fake `x-valtag-*` client headers + alternative carriers (`?key=`,
   `x-api-key`, `mj-api-secret`) → stripped or denied — verified with an
   echo upstream.
