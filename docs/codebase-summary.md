# Codebase Summary

## Structure

```
vaultsaas/
├── server/                  # Go 1.22 backend
│   ├── cmd/
│   │   ├── server/          # Main entry point (main.go)
│   │   ├── migrate/         # Migration runner (stub)
│   │   └── seed/            # Data seeder (stub)
│   ├── internal/
│   │   ├── auth/            # Argon2id, JWT RS256, middleware, handlers, Google OAuth2 (Phase 1.3/1.5)
│   │   ├── vault/           # MinIO storage, service CRUD, REST handler (Phase 1.3)
│   │   ├── middleware/      # Security headers, rate limiter (Phase 1.3)
│   │   ├── database/        # audit.go — audit log writer (Phase 1.3)
│   │   ├── policy/          # Risk tier engine (Tier 1-4 by credential type) (Phase 1.4)
│   │   ├── audit/           # Structured logger, SHA-256 hash chain, GET /audit/logs (Phase 1.4)
│   │   ├── notify/          # SMTP email notifications, Slack/Telegram adapters, webhooks, action tokens (Phase 1.4/2)
│   │   ├── consent/         # User consent recording, POST /consent (Phase 1.4)
│   │   ├── workflow/        # Approval state machine, multi-step approval chains, policy enforcement; IsAssignedApprover; rejection_reason persistence; approver/owner access checks on GetRequest/Approve/Reject/IssueCredential; dual-auth (user+agent) on CreateRequest/GetCredential/RevokeCredential; project-scope guard for agent cross-user requests; migration 000025 makes requester_user_id nullable (Phase 1.4/13/hardening/p0-gap)
│   │   ├── config/          # GoogleClientID/Secret/RedirectURL, DashboardURL (Phase 1.5)
│   │   ├── org/             # Organization CRUD, membership management (Phase 8)
│   │   ├── workspace/       # Workspace CRUD under org (Phase 8)
│   │   ├── project/         # Project CRUD under workspace (Phase 8)
│   │   ├── agent/           # Agent identity, token issuance (SHA-256), AgentAuthMiddleware (Phase 9)
│   │   ├── scanner/         # Scan result + finding CRUD, 5 HTTP endpoints, GetScanProjectID, checkScanAccess, rbac.Middleware on project routes (Phase 10/hardening)
│   │   ├── dynsecret/       # Provider interface, PostgresProvider, lease management, AES-256-GCM config+credential encryption, GetLeaseProviderID, auto-expiry worker (Phase 11/hardening)
│   │   ├── rbac/            # Role permission matrix (owner/admin/member/viewer), ResourceScans+ResourceDynSecret constants, RBAC middleware; 400 on missing project_id (Phase 13/hardening)
│   │   ├── ratelimit/       # Redis sliding-window per-agent rate limiting (go-redis/v9); Middleware(rpm) gates on X-Agent-ID header; fail-open on Redis errors (Phase 13/hardening)
│   │   ├── usage/           # Usage tracking, free tier enforcement middleware, GET /orgs/{id}/usage (Phase 15)
│   │   └── policy/          # Custom approval policy resolver, 3-level hierarchy (secret → project → tier defaults), GET|PUT endpoints (Phase 2)
│   ├── pkg/
│   │   ├── apierror/        # Standard API error JSON (Phase 1.3)
│   │   ├── crypto/          # Storage key generation (Phase 1.3)
│   │   └── validator/       # Input validation helpers (Phase 1.3)
│   ├── internal/database/migrations/
│   │   └── 000028_telegram_link_tokens.{up,down}.sql # Telegram user link tokens (Phase 2)
│   │
│   ├── cmd/
│   │   ├── server/          # Main entry point
│   │   ├── migrate/         # Migration runner
│   │   ├── seed/            # Data seeder
│   │   └── valt/            # valt CLI binary (cobra) — login, secrets, agents, scan, run (Phase 14)
│   ├── go.mod               # chi/v5, go-chi/cors, golang-jwt/jwt/v5, google/uuid, minio-go/v7,
│   │   │                   #   golang.org/x/oauth2, testify/v1.9.0, go-redis/v9, cobra
│   ├── go.sum
│   └── Dockerfile
│
├── dashboard/               # Next.js 15 + TypeScript + Tailwind v4 (Phase 1.5)
│   ├── src/
│   │   ├── app/
│   │   │   ├── (auth)/login/        # Google sign-in page
│   │   │   ├── (dashboard)/
│   │   │   │   ├── secrets/         # Secrets list + create/edit dialog + detail page
│   │   │   │   ├── approvals/       # Tabbed approval list + approve/reject dialog
│   │   │   │   ├── audit/           # Paginated audit log table
│   │   │   │   ├── projects/        # Project list + create; [id]/settings policy editor (Phase 2)
│   │   │   │   ├── settings/        # User profile, sign-out, upgrade page with UsageBar (Phase 15)
│   │   │   │   ├── orgs/            # Org list + create (Phase 8)
│   │   │   │   ├── projects/        # Project list + create (Phase 8)
│   │   │   │   ├── agents/          # Agent list + create, detail + token management (Phase 9)
│   │   │   │   ├── scans/           # Scan list + scan detail/findings (Phase 10)
│   │   │   │   └── providers/       # Dynamic secret provider list + create (Phase 11)
│   │   │   ├── api/
│   │   │   │   ├── proxy/[...path]/ # BFF JWT-forwarding proxy to Go backend
│   │   │   │   └── auth/logout/     # Logout route (clears httpOnly cookie)
│   │   │   ├── layout.tsx           # Sidebar nav + Header with user dropdown
│   │   │   ├── page.tsx
│   │   │   └── globals.css          # Dark mode CSS variables (zinc/slate palette)
│   │   ├── components/
│   │   │   ├── ui/                  # shadcn/ui primitives (button, input, card, badge, etc.)
│   │   │   ├── policy-editor.tsx    # PolicyEditor component for custom approval policies (Phase 2)
│   │   │   ├── secret-policy-section.tsx # SecretPolicySection wrapper (Phase 2)
│   │   │   └── project-policy-section.tsx # ProjectPolicySection wrapper (Phase 2)
│   │   └── lib/
│   │       ├── api-client.ts        # Typed fetch wrapper around BFF proxy
│   │       └── utils.ts             # cn() and shared helpers
│   ├── src/__tests__/               # vitest + happy-dom (19 tests)
│   │   ├── lib/utils.test.ts        # cn() helper (7 tests)
│   │   └── lib/api-client.test.ts   # fetch mocking, error handling (12 tests)
│   ├── package.json         # next@15.1.0, react@19, tailwindcss@4, lucide-react, vitest
│   ├── next.config.ts
│   ├── tsconfig.json
│   ├── postcss.config.mjs
│   └── Dockerfile
│
├── mcp-server/              # Rust MCP server (Phase 1.6/10/12)
│   ├── src/
│   │   ├── main.rs          # Async tokio entry point, JSON-RPC 2.0 dispatcher, --transport/--port CLI flags
│   │   ├── error.rs         # Unified error type
│   │   ├── protocol.rs      # MCP protocol types
│   │   ├── config.rs        # Config loading (env vars)
│   │   ├── keychain.rs      # OS keychain auth token storage (keyring crate)
│   │   ├── client.rs        # reqwest HTTP client to Go backend
│   │   ├── crypto.rs        # AES-256-GCM decrypt (aes-gcm crate)
│   │   ├── tools.rs         # 8 MCP tools (Phase 10 added scan_secrets, store_secret; Phase 11 added request_dynamic_secret)
│   │   ├── resources.rs     # 3 MCP resources
│   │   ├── scanner.rs       # 17 regex patterns for secret scanning (AWS, GitHub, Stripe, OpenAI, etc.) (Phase 10)
│   │   ├── scanner_tools.rs # scan_secrets + store_secret implementations; path traversal prevention: rejects absolute paths, drive letters, "..", paths > 500 chars (Phase 10/hardening)
│   │   └── http.rs          # axum HTTP server: POST /mcp, GET /mcp/sse (SSE), Bearer auth (Phase 12)
│   ├── Cargo.toml
│   └── Dockerfile
│
├── valt-cli/                # valt CLI binary (Go, Cobra) — standalone tool for agent-free credential access (Phase 2)
│   ├── cmd/
│   │   └── main.go          # CLI entry point
│   ├── internal/
│   │   ├── keychain.go      # OS keychain integration (cross-platform)
│   │   ├── oauth.go         # Device-like OAuth flow with browser redirect
│   │   ├── mcp_installer.go # Auto-detect MCP server and update config
│   │   └── config.go        # Config at ~/.valt/config.json
│   ├── go.mod
│   ├── .goreleaser.yml      # Cross-platform binary builds (goreleaser)
│   └── README.md
│
├── sdk/
│   ├── go/                  # Go SDK (stdlib only, go.mod: github.com/ldphuong-vn/valtag-go) (Phase 14)
│   └── python/              # Python SDK (urllib only, no external deps) (Phase 14)
│
├── docs/                    # Project documentation
├── plans/                   # Implementation plans
├── scripts/
│   ├── setup-dev.sh         # Dev environment setup
│   └── gen-keys.sh          # RSA key pair generation (Phase 1.3)
├── keys/                    # Local dev keys (gitignored)
├── docker-compose.yml       # Dev environment
├── docker-compose.prod.yml  # Production environment
├── Caddyfile                # Dev reverse proxy (CSP, security headers)
├── Caddyfile.prod           # Prod reverse proxy
├── Makefile                 # Common commands
├── .env.example             # Environment variable template
├── CLAUDE.md                # AI assistant instructions
├── SECURITY.md              # Security policy
└── README.md
```

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.22, chi/v5 v5.1.0, go-chi/cors v1.2.1, golang-jwt/jwt/v5, google/uuid, minio-go/v7, golang.org/x/oauth2, testify v1.9.0, go-redis/v9, cobra |
| Frontend | Next.js 15.1.0, React 19, TypeScript 5.7, Tailwind v4, shadcn/ui, lucide-react, Geist fonts, vitest |
| MCP Server | Rust (tokio async, serde, reqwest, keyring, aes-gcm, axum, clap — see Cargo.toml) |
| CLI | valt CLI (cobra), Go SDK (stdlib), Python SDK (urllib) |
| Proxy | Caddy (CSP headers, HSTS, security headers) |
| Cache | Redis (optional, REDIS_URL env var) — per-agent rate limiting |
| Infra | Docker Compose (dev + prod), Makefile |

## Implementation Status

| Phase | Status |
|---|---|
| 1.1 Scaffolding & Infrastructure | DONE — 2026-03-16 |
| 1.2 Database Layer & Migrations | DONE — 2026-03-16 |
| 1.3 Backend Core (auth, vault, middleware) | DONE — 2026-03-16 |
| 1.4 Backend Workflow + Audit + Policy + Notify + Consent | DONE — 2026-03-17 |
| 1.5 Next.js Dashboard + Google OAuth | DONE — 2026-03-17 |
| 1.6 Rust MCP Server | DONE — 2026-03-17 |
| 1.7 Testing & Hardening | DONE — 2026-03-17 |
| 8 Organization Hierarchy | DONE — 2026-03-17 |
| 9 AI Agent Identity | DONE — 2026-03-17 |
| 10 Secret Scanner | DONE — 2026-03-17 |
| 11 Dynamic Secrets | DONE — 2026-03-17 |
| 12 MCP Gateway (HTTP/SSE) | DONE — 2026-03-17 |
| 13 Enhanced RBAC + Policies | DONE — 2026-03-17 |
| 14 CLI + SDKs | DONE — 2026-03-17 |
| 15 Cloud Free Tier | DONE — 2026-03-17 |
| 16 Security Hardening | DONE — 2026-03-17 |
| P0 Gap: Agent Cross-User Access Request | DONE — 2026-03-18 |
| 2 Approval Channels + Custom Policies + CLI | DONE — 2026-03-19 |

- **server**: Phase 1.3-1.5 + Phase 2 complete; Google OAuth2, org/workspace/project hierarchy, agent identity, secret scanner, dynamic secrets, RBAC, rate limiting, usage/free tier, approval channels (email action tokens, Slack Block Kit, Telegram bot), custom approval policies (3-level hierarchy: secret → project → tier defaults), policy GET|PUT endpoints; security hardening (AES-256-GCM on provider configs + lease credentials, RBAC, rate limiting, rejection_reason persistence); migrations 000001-000028; 74+ Go unit tests passing
- **dashboard**: Full Next.js App Router — sign-in, secrets CRUD, approvals, audit, orgs, projects (with policy editor), agents, scans, providers, settings/upgrade; BFF proxy; PolicyEditor, SecretPolicySection, ProjectPolicySection components; 19 vitest tests passing
- **mcp-server**: 8 MCP tools, 3 resources; stdio + HTTP/SSE dual transport (axum); Bearer auth; 17 regex scanner patterns; OS keychain auth; 11+ unit tests passing
- **sdk**: Go SDK (`github.com/ldphuong-vn/valtag-go`, stdlib only) and Python SDK (urllib only)
- **cli**: `valt-cli` binary (Go, Cobra) standalone agent-free credential access; OAuth device-like flow with browser redirect, OS keychain integration, auto-detect MCP server for installation, config at `~/.valt/config.json`, cross-platform builds via goreleaser
- **testing**: Makefile targets `test-unit`, `test-integration`, `test-dashboard`, `test-mcp`, `security`; `.golangci.yml` enabled
- **infra**: Full Docker Compose (dev/prod) with JWT key volume mount, optional Redis, Caddy with CSP/security headers
