# System Architecture

## Overview

```
Client Layer:  MCP Server (Rust, stdio) + Dashboard (Next.js)
    ↓ HTTPS
Backend:       Go Monolith (chi/v5 router)
               - Auth, Vault, Workflow, Audit, Notify, Org, Agent, Gateway modules
    ↓
Data Layer:    PostgreSQL 16 (metadata + audit) + MinIO (encrypted blobs)
    ↑
Proxy:         Caddy (reverse proxy + auto TLS)
    ↑
Gateway:       Go HTTP Forward Proxy (:10256, optional)
               - Transparent credential injection for any agent framework
```

## Components

| Component | Tech | Purpose |
|-----------|------|---------|
| server/ | Go 1.22+ | REST API monolith |
| dashboard/ | Next.js 15 | Web UI (SSR, client-side crypto) |
| mcp-server/ | Rust | Local MCP server for AI agents |
| PostgreSQL | v16 | Metadata, audit logs (partitioned) |
| MinIO | Latest | S3-compatible encrypted blob storage |
| Caddy | v2 | Reverse proxy, auto TLS |

## Key Decisions

| Decision | Rationale |
|----------|-----------|
| Go monolith over microservices | Simpler deploy/debug for MVP |
| Caddy over Kong | Auto TLS, simple config |
| PostgreSQL for audit logs | Partitioned tables sufficient for MVP |
| JWT RS256 over Keycloak | Fewer infra dependencies |
| State machine over Temporal | Approval workflow is simple enough |
| Docker Compose over K8s | Single-region MVP |

## Dashboard Architecture (Phase 1.5)

**Pattern:** Next.js App Router with BFF (Backend For Frontend) proxy.

```
Browser
  └─▶ Next.js Server Components (SSR)
        └─▶ api/proxy/[...path]  ──HTTPS──▶  Go backend (JWT in Authorization header)
              ▲
              │ httpOnly cookie (token) set by Go OAuth callback
```

- All Go API calls route through `api/proxy/[...path]` — the BFF reads the httpOnly cookie and forwards `Authorization: Bearer <token>` to Go.
- `api/auth/logout` clears the httpOnly cookie server-side.
- Google OAuth: browser → `GET /api/v1/auth/google` (Go redirects to Google) → Google callback → `GET /api/v1/auth/google/callback` (Go upserts user, sets cookie, redirects to dashboard).

### Route Groups

| Group | Routes | Description |
|-------|--------|-------------|
| `(auth)` | `/login` | Google sign-in landing |
| `(dashboard)` | `/secrets`, `/approvals`, `/audit`, `/settings`, `/orgs`, `/projects`, `/agents`, `/agents/[id]` | Authenticated pages with Sidebar + Header layout |

## MCP Server Architecture (Phase 1.6)

**Pattern:** Async tokio process reading JSON-RPC 2.0 from stdin, writing to stdout.

```
AI Agent (Claude, etc.)
  └─▶ stdio JSON-RPC 2.0
        └─▶ mcp-server (Rust, tokio)
              ├─ tools.rs   (5 tools)
              ├─ resources.rs (3 resources)
              └─ client.rs  ──HTTPS──▶  Go backend
```

### MCP Tools

| Tool | Description |
|------|-------------|
| `request_secret_access` | Submit approval request for a secret |
| `check_approval_status` | Poll approval state by request ID |
| `get_credential` | Retrieve decrypted credential after approval |
| `revoke_credential` | Revoke an active credential session |
| `list_my_secrets` | List secrets owned by the authenticated agent |

### MCP Resources

| Resource URI | Description |
|-------------|-------------|
| `vault://secrets` | Index of accessible secrets |
| `vault://requests/{id}` | Approval request detail |
| `vault://audit/today` | Today's audit log entries |

### MCP Tools (Phase 9 addition)

| Tool | Description |
|------|-------------|
| `authenticate_agent` | Accept an agent token, persist to OS keychain, clear env var |

### Auth Token Storage
1. OS keychain via `keyring` crate (primary)
2. `VALT_TOKEN` env var / `VALTAG_AGENT_TOKEN` env var (fallback)

## valtag CLI (Phase 2)

**Pattern:** Standalone Go binary for agent-free credential access; cross-platform via goreleaser.

```
Developer Machine
  └─▶ valt-cli (macOS/Linux/Windows binary)
        ├─ Device-like OAuth flow (browser redirect, validates callback on localhost)
        ├─ OS keychain: store OAuth token + agent token (macOS: Keychain, Linux: pass, Windows: Credential Manager)
        ├─ Auto-detect MCP server and update ~/.mcp/claude.json for seamless MCP install
        └─ Config: ~/.valtag/config.json (auth state, server URL)
```

- **OAuth flow**: CLI opens browser, authenticates user, receives temporary code → exchanges for token, stores in keychain
- **Keychain integration**: Cross-platform (Keychain, pass, Credential Manager) via `internal/keychain.go`
- **MCP installer**: Detects running MCP server, auto-updates `.mcp/claude.json` config to include valtag
- **Binary distribution**: `.goreleaser.yml` builds amd64/arm64 for macOS/Linux/Windows; published on GitHub Releases

## Test Architecture (Phase 1.7)

| Layer | Runner | Count | Location |
|-------|--------|-------|----------|
| Go unit | `go test ./...` | 74 | `server/internal/*/`, `server/pkg/*/` |
| Dashboard unit | `vitest` | 19 | `dashboard/src/lib/__tests__/` |
| Rust unit | `cargo test` | 11 | `mcp-server/src/*.rs` |

**Make targets:** `test-unit`, `test-integration`, `test-dashboard`, `test-mcp`, `security`

Go linting via `.golangci.yml` (govet shadow, errcheck, staticcheck, unused).

## Go Backend Packages

### Phase 1.3

| Package | Purpose |
|---------|---------|
| `internal/auth/` | Argon2id hashing, JWT RS256 issuance/validation, auth middleware, register/login/refresh handlers |
| `internal/vault/` | MinIO storage layer, vault service (CRUD), REST handler |
| `internal/middleware/` | Security headers, sliding-window rate limiter |
| `internal/database/audit.go` | Audit log writer |
| `pkg/apierror/` | Standard API error JSON response format |
| `pkg/validator/` | Input validation helpers |
| `pkg/crypto/` | Storage key generation |

### Phase 1.4

| Package | Purpose |
|---------|---------|
| `internal/policy/engine.go` | Risk tier classification — `api_key`=Tier1, `db_credential`/`ssh_key`/`oauth_token`=Tier2, `cloud_credential`/`personal_session`=Tier3, unknown=Tier4 |
| `internal/audit/logger.go` | Structured audit log writer |
| `internal/audit/hash-chain.go` | SHA-256 hash chain — links each audit record to its predecessor |
| `internal/audit/handler.go` | `GET /api/v1/audit/logs` handler |
| `internal/notify/service.go` | Notification dispatcher |
| `internal/notify/email.go` | SMTP email delivery; no-op when SMTP not configured |
| `internal/consent/service.go` | Consent record creation and lookup |
| `internal/consent/handler.go` | `POST /api/v1/consent` handler |
| `internal/workflow/service.go` | Approval state machine: pending → approved/rejected → active → expired/revoked; `IsAssignedApprover` check |
| `internal/workflow/credential.go` | Temporary credential lifecycle (issue, expire, revoke) |
| `internal/workflow/handler.go` | 6 workflow endpoints; approver/owner access enforcement on GetRequest, Approve, Reject, IssueCredential; surfaces IssueCredential errors |
| `internal/workflow/approval-chain.go` | Multi-step approval chain; `RejectionReason *string` field on `ApprovalStep`; persists rejection reason via `AdvanceChain` in a serializable transaction |

## Workflow State Machine (Phase 1.4)

```
                  ┌─────────┐
    create ──────▶│ pending │
                  └────┬────┘
           approve ────┤──── reject
                  ┌────▼────┐   ┌──────────┐
                  │approved │   │ rejected │
                  └────┬────┘   └──────────┘
              activate │
                  ┌────▼────┐
                  │ active  │
                  └────┬────┘
          expire / ────┤──── revoke
         natural TTL   │
              ┌────────┴──────────┐
         ┌────▼────┐        ┌─────▼────┐
         │ expired │        │ revoked  │
         └─────────┘        └──────────┘
```

### Policy Rules (enforced per tier)

| Rule | Tier 1 | Tier 2 | Tier 3 |
|------|--------|--------|--------|
| Auto-approve | Yes | No | No |
| Single-use session | No | No | Yes |
| Duration cap | 24h | 8h | 1h |
| Daily request limit | 100 | 20 | 5 |
| Cool-down between requests | None | 1h | 4h |
| Reason min length | 0 | 20 chars | 50 chars |

### Phase 1.5 (OAuth additions)

| Package | Purpose |
|---------|---------|
| `internal/auth/oauth.go` | Google OAuth2 flow — redirect, callback, user upsert, httpOnly cookie issuance |
| `internal/config/config.go` | `GoogleClientID`, `GoogleClientSecret`, `GoogleRedirectURL`, `DashboardURL` config fields |

### Phase 8 (Org hierarchy)

| Package | Purpose |
|---------|---------|
| `internal/org/` | Organization CRUD, membership management, org-scoped JWT claims |
| `internal/workspace/` | Workspace CRUD scoped to org |
| `internal/project/` | Project CRUD scoped to workspace, project membership |

### Phase 9 (Agent identity)

| Package | Purpose |
|---------|---------|
| `internal/agent/service.go` | Agent identity lifecycle — create, rotate tokens, deactivate |
| `internal/agent/handler.go` | REST handlers for agent and token endpoints |
| `internal/agent/middleware.go` | Agent token auth middleware (validates `agent_tokens` table) |

### Proxy Gateway

| Package | Purpose |
|---------|---------|
| `internal/gateway/server.go` | HTTP forward proxy — agent auth, route matching, credential injection, CONNECT tunneling |
| `internal/gateway/store.go` | DB queries for `proxy_routes` and `proxy_endpoint_limits` tables |
| `internal/gateway/injector.go` | Credential injection logic, glob path matching, placeholder scanning |
| `internal/gateway/handler.go` | REST CRUD handlers for proxy routes and endpoint limits |

### Phase 2 (Approval channels + custom policies)

| Package | Purpose |
|---------|---------|
| `internal/notify/action_token.go` | Email action token generation & validation (HMAC-signed, short-lived) |
| `internal/notify/channel_handler.go` | Notification channel registry & delivery dispatcher |
| `internal/notify/channel_store.go` | In-memory store for approval action mappings (request_id → approval action) |
| `internal/notify/slack.go` | SlackAdapter: Block Kit interactive messages with Approve/Reject buttons |
| `internal/notify/slack_webhook.go` | SlackWebhookHandler: HMAC-SHA256 signature verification, button callback routing |
| `internal/notify/telegram.go` | TelegramAdapter: Inline keyboard buttons, /start linking flow for account association |
| `internal/notify/telegram_webhook.go` | TelegramWebhookHandler: Bot API callbacks, Telegram user → valtag user mapping |
| `internal/policy/resolver.go` | Custom policy resolver: 3-level hierarchy (secret → project → tier defaults) |
| `internal/vault/handler.go` (new) | `GetSecretPolicy`, `PutSecretPolicy` handlers — secret-level policy CRUD |
| `internal/project/handler.go` (new) | `GetProjectPolicy`, `PutProjectPolicy` handlers — project-level policy CRUD |

## API Routes

### Phase 1.3

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| POST | `/api/v1/auth/register` | — | Register user |
| POST | `/api/v1/auth/login` | — | Login, return tokens |
| POST | `/api/v1/auth/refresh` | — | Refresh access token |
| GET | `/api/v1/secrets` | JWT | List secrets (paginated) |
| POST | `/api/v1/secrets` | JWT | Create secret |
| GET | `/api/v1/secrets/{id}` | JWT | Get secret by ID |
| PUT | `/api/v1/secrets/{id}` | JWT | Update secret |
| DELETE | `/api/v1/secrets/{id}` | JWT | Delete secret |

### Phase 1.4

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| POST | `/api/v1/requests` | JWT | Create access request |
| GET | `/api/v1/requests` | JWT | List access requests |
| GET | `/api/v1/requests/{id}` | JWT | Get request by ID |
| POST | `/api/v1/requests/{id}/approve` | JWT | Approve request |
| POST | `/api/v1/requests/{id}/reject` | JWT | Reject request |
| POST | `/api/v1/requests/{id}/revoke` | JWT | Revoke active session |
| GET | `/api/v1/audit/logs` | JWT | List audit log entries |
| POST | `/api/v1/consent` | JWT | Record user consent |

### Phase 1.5

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| GET | `/api/v1/auth/google` | — | Initiate Google OAuth2 flow |
| GET | `/api/v1/auth/google/callback` | — | Google OAuth callback; upsert user, set httpOnly cookie |

### Phase 8 (Org hierarchy)

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| GET | `/api/v1/orgs` | JWT | List orgs for current user |
| POST | `/api/v1/orgs` | JWT | Create org |
| GET | `/api/v1/orgs/{id}` | JWT | Get org |
| PUT | `/api/v1/orgs/{id}` | JWT | Update org |
| DELETE | `/api/v1/orgs/{id}` | JWT | Delete org |
| GET | `/api/v1/orgs/{id}/members` | JWT | List org members |
| POST | `/api/v1/orgs/{id}/members` | JWT | Add member to org |
| DELETE | `/api/v1/orgs/{id}/members/{uid}` | JWT | Remove member from org |
| GET | `/api/v1/orgs/{id}/workspaces` | JWT | List workspaces in org |
| POST | `/api/v1/orgs/{id}/workspaces` | JWT | Create workspace |
| GET | `/api/v1/workspaces/{id}` | JWT | Get workspace |
| PUT | `/api/v1/workspaces/{id}` | JWT | Update workspace |
| DELETE | `/api/v1/workspaces/{id}` | JWT | Delete workspace |
| GET | `/api/v1/workspaces/{id}/projects` | JWT | List projects in workspace |
| POST | `/api/v1/workspaces/{id}/projects` | JWT | Create project |
| GET | `/api/v1/projects/{id}` | JWT | Get project |
| PUT | `/api/v1/projects/{id}` | JWT | Update project |
| DELETE | `/api/v1/projects/{id}` | JWT | Delete project |
| GET | `/api/v1/projects/{id}/members` | JWT | List project members |
| POST | `/api/v1/projects/{id}/members` | JWT | Add member to project |
| DELETE | `/api/v1/projects/{id}/members/{uid}` | JWT | Remove member from project |

### Phase 9 (Agent identity)

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| GET | `/api/v1/projects/{id}/agents` | JWT | List agents in project |
| POST | `/api/v1/projects/{id}/agents` | JWT | Create agent identity |
| GET | `/api/v1/agents/{id}` | JWT | Get agent |
| PUT | `/api/v1/agents/{id}` | JWT | Update agent |
| DELETE | `/api/v1/agents/{id}` | JWT | Delete agent |
| GET | `/api/v1/agents/{id}/tokens` | JWT | List agent tokens |
| POST | `/api/v1/agents/{id}/tokens` | JWT | Issue new agent token |
| DELETE | `/api/v1/agents/{id}/tokens/{tid}` | JWT | Revoke agent token |

### Proxy Gateway (credential injection)

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| GET | `/api/v1/proxy-routes?agent_id=X` | JWT | List proxy routes for agent |
| POST | `/api/v1/proxy-routes` | JWT | Create proxy route (auto-generates placeholder) |
| PUT | `/api/v1/proxy-routes/{id}` | JWT | Update proxy route |
| DELETE | `/api/v1/proxy-routes/{id}` | JWT | Delete proxy route |
| GET | `/api/v1/proxy-endpoint-limits?agent_id=X` | JWT | List endpoint rate limits |
| POST | `/api/v1/proxy-endpoint-limits` | JWT | Create endpoint rate limit |
| DELETE | `/api/v1/proxy-endpoint-limits/{id}` | JWT | Delete endpoint rate limit |

### Phase 2 (Approval channels + custom policies + CLI)

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| POST | `/api/v1/me/telegram-link` | JWT | Generate Telegram link token, user /start links account |
| GET | `/api/v1/secrets/{id}/policy` | JWT | Get custom policy for secret |
| PUT | `/api/v1/secrets/{id}/policy` | JWT | Set custom policy for secret |
| GET | `/api/v1/projects/{project_id}/policy` | JWT | Get custom policy for project |
| PUT | `/api/v1/projects/{project_id}/policy` | JWT | Set custom policy for project |
| GET | `/api/v1/credentials/active` | Agent | List active credentials (agent use) |
| POST | `/webhooks/slack/interactions` | Slack Signature | Handle Slack button clicks (approve/reject) |
| POST | `/webhooks/telegram` | Telegram Secret Token | Handle Telegram bot messages (approval buttons, /start) |

## Agent Auth Flow (Phase 9)

```
MCP Server (Rust)
  └─▶ VALTAG_AGENT_TOKEN env var (or OS keychain via authenticate_agent tool)
        └─▶ Go backend — agent token middleware
              └─▶ validates token hash against agent_tokens table
                    └─▶ attaches agent identity to request context
```

- Agent tokens are stored as hashed values in `agent_tokens` table; plaintext returned only at creation.
- `authenticate_agent` MCP tool accepts a token, stores it in the OS keychain, and clears `VALTAG_AGENT_TOKEN` env usage.

## Approval Channels (Phase 2)

Approvers receive multi-channel notifications with action tokens or interactive controls:

### Email (Phase 1.4)
- Action token links: `POST /api/v1/requests/{id}/approve?token=XXXXX` (short-lived, HMAC-signed)
- SMTP delivery via `internal/notify/email.go`
- Fallback: no-op when SMTP not configured

### Slack (Phase 2)
- SlackAdapter: Block Kit interactive message (`POST /webhooks/slack/interactions`)
- Approval buttons (Approve/Reject) trigger webhook callback with action_id
- `slack_webhook.go`: Verifies Slack signature (HMAC-SHA256), routes to approval endpoint
- Requires: `SLACK_SIGNING_SECRET`, `SLACK_BOT_TOKEN` env vars

### Telegram (Phase 2)
- TelegramAdapter: Inline keyboard buttons + /start linking flow
- Webhook endpoint: `POST /webhooks/telegram` (Telegram API callback)
- Link token flow: `POST /me/telegram-link` generates link token, `/start {token}` on Telegram links user account
- Approval buttons resolve to `POST /api/v1/requests/{id}/approve` with Bearer token from Telegram user mapping
- Requires: `TELEGRAM_BOT_TOKEN` env var; `telegram_link_tokens` table stores temp link tokens

### Action Token Flow
```
Approver ◀─── Notification (email/Slack/Telegram) ◀─── request pending
   │
   └──▶ Click Approve ──▶ POST /api/v1/requests/{id}/approve
         (token or auth) ──▶ Validate token/auth
                            Update request state → approved
                            Issue credential
```

## Custom Approval Policies (Phase 2)

Policies enforce approval rules per-secret and per-project. Three-level resolution hierarchy:

1. **Secret-level policy** (if set): `GET|PUT /api/v1/secrets/{id}/policy`
   - Overrides project policy for that secret
   - Properties: `require_reason`, `auto_approve`, `approval_steps`, `duration_minutes`

2. **Project-level policy** (if set): `GET|PUT /api/v1/projects/{project_id}/policy`
   - Applies to all secrets in project (unless overridden)
   - Same properties as secret policy

3. **Tier defaults** (Phase 1.4)
   - Fallback when no secret/project policy exists
   - Based on credential risk tier (Tier 1-4)
   - Example: Tier 3 requires approver chain, max 1-hour duration

### Dashboard UI (Phase 2)
- `PolicyEditor` component: Edit approval steps, reason requirements, duration, auto-approve toggle
- `SecretPolicySection`: Per-secret policy editor (secrets detail page)
- `ProjectPolicySection`: Per-project policy editor (project settings page)

## Security Architecture

### Encryption at Rest
- **Secret values**: Envelope encryption — random DEK (AES-256-GCM) per secret, DEK wrapped by master KEK (`VAULT_MASTER_KEY` env var). Pattern mirrors AWS Secrets Manager / HashiCorp Vault.
- **Provider configs** (`dynamic_providers.config_enc`): AES-256-GCM, keyed by `masterKey []byte` injected into `dynsecret.Service` at startup. Plaintext JSON fallback on decrypt failure for pre-migration rows.
- **Lease credentials** (`dynamic_leases.secret_data_enc`): AES-256-GCM, same `masterKey`. Decrypted on read in `RevokeLease`; plaintext fallback for legacy rows.
- **wiring**: `cmd/server/main.go` passes master key to `dynsecret.NewService(db, masterKey)`.

### Authentication
- JWT RS256 (15min access / 7day refresh)
- Agent tokens: SHA-256 hash stored in `agent_tokens`; `X-Agent-ID` header identifies agent for rate limiting

### RBAC
`rbac.Middleware(db, projectParam, resource, action)` resolves the caller's project role from `project_memberships` and calls `rbac.Can(role, resource, action)`. Returns 400 if `project_id` URL param is absent; 403 if not a member or insufficient role.

| Resource constant | Applied to |
|---|---|
| `ResourceSecret` | vault routes |
| `ResourceProject` | project routes |
| `ResourceAgent` | agent routes |
| `ResourceScans` | scanner project routes |
| `ResourceDynSecret` | dynsecret project routes |

Role permission matrix (owner/admin have identical rights):

| Resource | owner/admin | member | viewer |
|---|---|---|---|
| secret | read, write, admin, approve | read, write | read |
| project | read, write, admin | read | read |
| agent | read, write, admin | read, write | read |
| scans | read, write | read, write | read |
| dynsecret | read, write | read | read |

### Rate Limiting
`ratelimit.RedisLimiter.Middleware(rpm)` gates on `X-Agent-ID` request header. Requests without the header (human dashboard users) pass through unconditionally. On Redis error the middleware is fail-open (allows the request). Returns HTTP 429 with `{"error":"rate limit exceeded"}` when the sliding window is exceeded.

### Other Controls
- Sliding-window rate limiting on all routes (general middleware layer)
- Security headers enforced via `internal/middleware/`
- Audit hash chain (SHA-256) — each record links to predecessor
- Path traversal prevention in `mcp-server/scanner_tools.rs`: rejects absolute paths (`/`, `\`, drive letters), `..` sequences, and paths longer than 500 characters
