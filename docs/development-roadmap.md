# Development Roadmap

## Phase 1: MVP (Current)

### Phase 1.1: Scaffolding & Infrastructure [DONE - 2026-03-16]
- [x] Monorepo structure
- [x] Docker Compose (dev + prod)
- [x] Service stubs (Go, Next.js, Rust)
- [x] Documentation

### Phase 1.2: Database Layer & Migrations [Planned]
- [ ] PostgreSQL connection pool
- [ ] All 6 tables (users, secrets, access_requests, credential_sessions, audit_logs, refresh_tokens)
- [ ] Migration runner
- [ ] Seed script

### Phase 1.3: Backend Core [DONE - 2026-03-16]
- [x] Auth (register, login, refresh, JWT RS256, Argon2id)
- [x] Vault (CRUD, MinIO storage, envelope encryption)
- [x] Middleware (rate limit, CORS, security headers)

### Phase 1.4: Backend Workflow + Audit + Policy + Notify + Consent [DONE - 2026-03-17]
- [x] DB migrations 000007-000011 (secrets, access_requests, credential_sessions, audit_logs, user_consent_logs)
- [x] Policy engine — risk tier system (Tier 1-4 by credential type)
- [x] Audit logging with SHA-256 hash chain, GET /audit/logs endpoint
- [x] SMTP email notification service
- [x] User consent recording, POST /consent endpoint
- [x] Approval state machine (pending→approved/rejected→active→expired/revoked), 6 workflow endpoints
- [x] Policy enforcement (duration cap, reason length, daily limit, cool-down, auto-approve Tier 1, single-use Tier 3)

### Phase 1.5: Next.js Dashboard + Google OAuth [DONE - 2026-03-17]
- [x] Google OAuth2 flow (Go backend: `oauth.go`, migration 000012, `golang.org/x/oauth2`)
- [x] Auth page — Google sign-in (`(auth)/login`)
- [x] Secrets CRUD — list table, create/edit dialog, detail page
- [x] Approvals — tabbed list (All/Pending/Approved/Rejected), approve/reject dialog
- [x] Audit log viewer — paginated table
- [x] Settings — user profile + sign-out
- [x] BFF proxy (`api/proxy/[...path]`) for JWT forwarding
- [x] shadcn/ui primitives, Sidebar + Header layout, dark mode

### Phase 1.6: Rust MCP Server [DONE - 2026-03-17]
- [x] Async tokio runtime
- [x] 5 MCP tools: `request_secret_access`, `check_approval_status`, `get_credential`, `revoke_credential`, `list_my_secrets`
- [x] 3 MCP resources: `vault://secrets`, `vault://requests/{id}`, `vault://audit/today`
- [x] OS keychain auth token storage (keyring crate), env var fallback
- [x] reqwest HTTP client to Go backend, AES-256-GCM decrypt stub
- [x] `cargo clippy -- -D warnings` clean

### Phase 1.7: Testing & Hardening [DONE - 2026-03-17]
- [x] 74 Go unit tests passing (auth, vault, policy, audit, middleware, validator, apierror, crypto)
- [x] 19 dashboard vitest tests passing (utils, api-client)
- [x] 11 Rust unit tests passing
- [x] Makefile targets: `test-unit`, `test-integration`, `test-dashboard`, `test-mcp`, `security`
- [x] `.golangci.yml`: govet shadow, errcheck, staticcheck, unused enabled

### Phase 1.8: Security Hardening [DONE - 2026-03-17]
- [x] AES-256-GCM encryption for dynamic provider configs and lease credentials at rest
- [x] RBAC authorization on all scanner routes (createScan, listScans, listFindings, importFinding, dismissFinding)
- [x] RBAC authorization on all dynsecret routes (createProvider, listProviders, createLease, listLeases, revokeLease)
- [x] `ResourceScans` + `ResourceDynSecret` added to RBAC permission matrix
- [x] Org-scoped usage counts (secrets, agents) — fixed global COUNT replaced with 3-table JOIN scoped to org_id
- [x] Approval access: `GetRequest`, `Approve`, `Reject` allow assigned approvers + secret owners (not requester-only)
- [x] Auth rate limiting middleware (`ratelimit/middleware.go`, 60rpm per agent via X-Agent-ID, Redis ZADD dedup fixed)
- [x] `Approve` handler propagates `IssueCredential` failure as HTTP 500
- [x] MCP scanner path validation: rejects absolute paths, Windows drive letters, `..` sequences, paths > 500 chars
- [x] Migration `000024`: `rejection_reason TEXT` on `approval_steps`; `AdvanceChain` persists rejection reason
- [x] RBAC middleware returns HTTP 400 on missing project_id (was silent pass-through)
- [x] `VAULT_MASTER_KEY` documented in `.env.example` and `docker-compose.yml`

### Phase 8: Organization Hierarchy [DONE - 2026-03-17]
- [x] DB migrations 013-015: organizations, workspaces, projects, memberships, secrets.project_id
- [x] Go packages: `internal/org/`, `internal/workspace/`, `internal/project/`
- [x] API routes: /orgs, /orgs/{id}/members, /orgs/{id}/workspaces, /workspaces/{id}/projects, /projects/{id}, /projects/{id}/members
- [x] Dashboard: org context in Sidebar, /orgs page, /projects page

### Phase 9: AI Agent Identity [DONE - 2026-03-17]
- [x] DB migrations 016-017: agent_identities, agent_tokens tables
- [x] Go package: `internal/agent/` (service, handler, middleware)
- [x] API routes: /projects/{id}/agents, /agents/{id}, /agents/{id}/tokens, /agents/{id}/tokens/{tid}
- [x] MCP server: `authenticate_agent` tool, VALTAG_AGENT_TOKEN env, agent token keychain storage
- [x] Dashboard: /agents page (list + create), /agents/[id] page (detail + token management)

### Phase 10: E2E Credential Delivery [DONE - 2026-03-17]
- [x] server/pkg/crypto/aes.go — AES-256-GCM encrypt/decrypt, GenerateDEK
- [x] config: VAULT_MASTER_KEY with cached ephemeral fallback
- [x] vault/handler.go — accept plaintext value, server-side envelope encryption on create
- [x] vault/service.go — GetSecretByID (no owner constraint), GetBlob
- [x] workflow/handler.go — GetCredential decrypts blob and returns value; RevokeCredential ownership check; Approve uses GetSecretByID; GetRequest endpoint added
- [x] workflow/credential.go — CredentialSession.Value field
- [x] GET /access-requests/{request_id} route added
- [x] mcp-server: get_credential returns value; check_approval_status uses single-request endpoint
- [x] Dashboard: Credential type aligned to Go JSON tags; AccessRequest field names fixed; approval table duration and secret name display fixed

### P0 Gap: Agent Cross-User Access Request [DONE - 2026-03-18]
- [x] `dualAuthMiddleware` wired in `main.go`; workflow routes accept both user JWT and agent token
- [x] `WithUserID` / `WithAgentID` context setters added to auth/agent middleware
- [x] `vault/service.go`: `Secret.ProjectID` field added; `GetSecretByID` selects `project_id`
- [x] `workflow/handler.go`: project-scope guard (`agent.project_id == secret.project_id`); dual-auth in `CreateRequest`, `GetCredential`, `RevokeCredential`, `GetRequest`
- [x] `workflow/service.go`: dual-path rate limit; COALESCE scan on list query
- [x] Migration `000025`: `requester_user_id` made nullable (supports agent-only requests)
- [x] `mcp-server/src/client.rs`: `create_access_request` sends `requester_type: "ai_agent"`
- [x] 30 new tests pass

## Phase 2: Product-Market Fit [DONE - 2026-03-19]

### Phase 2.1: Approval Channels (Ph01-04) [DONE - 2026-03-19]
- [x] Email action links (one-click approve/reject), request_action_tokens table
- [x] Notification channel settings (DB schema, API, dashboard UI)
- [x] Slack bot (Block Kit, HMAC-SHA256 webhook, interactivity handler)
- [x] Telegram bot (inline keyboard, deep-link account linking, telegram_link_tokens table)

### Phase 2.2: Custom Policies (Ph01-03) [DONE - 2026-03-19]
- [x] DB migrations: policy_config JSONB on secrets + projects tables
- [x] Three-level policy resolver (secret → project → tier defaults)
- [x] GET/PUT /secrets/{id}/policy and /projects/{project_id}/policy endpoints
- [x] Dashboard PolicyEditor UI with per-secret and per-project settings pages

### Phase 2.3: valt CLI (Ph01-05) [DONE - 2026-03-19]
- [x] Project scaffold with config/keychain/API client (Cobra, go-keyring, toml)
- [x] `valt setup` & `valt mcp install` (OAuth CLI flow, IDE config generation)
- [x] `valt list` / `valt get` / `valt run` (secret listing, retrieval, env injection)
- [x] `valt request` & `valt status` (access request creation and polling)
- [x] goreleaser config & CI release workflow (cross-platform builds: Linux/Mac/Windows)

## Phase 3: Scale [Future]
- Kubernetes migration
- Multi-region
- SSO (OIDC)
