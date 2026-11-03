# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
valtag is an MCP-native secret vault with human-in-the-loop approval for AI agents. Monorepo: Go backend, Next.js dashboard, Rust MCP server.

## Commands

```bash
# Full dev environment (Docker required)
make dev              # Start all services (server, dashboard, postgres, minio, caddy)
make down             # Stop all services
make clean            # Remove all volumes and containers

# Testing
make test             # All tests (Go unit + dashboard + Rust)
make test-unit        # Go unit tests only (no DB): cd server && go test ./internal/... ./pkg/...
make test-integration # Go integration tests (requires Docker): cd server && go test ./tests/integration/...
make test-dashboard   # Dashboard: cd dashboard && npm test
make test-mcp         # Rust: cd mcp-server && cargo test

# Run a single Go test
cd server && go test ./internal/workflow/... -run TestApproveRequest -v

# Run a single dashboard test
cd dashboard && npx vitest run src/path/to/test.ts

# Linting
make lint             # All linters
cd server && golangci-lint run ./...
cd dashboard && npm run lint
cd mcp-server && cargo clippy -- -D warnings

# Database
make migrate-up       # Run migrations (via Docker)
make migrate-down     # Rollback last migration
make seed             # Seed dev data
make db-shell         # psql shell

# Key generation
openssl rand -base64 32   # Generate VAULT_MASTER_KEY
bash scripts/gen-keys.sh  # Generate JWT RS256 key pair
```

## Architecture

### Request flow
Dashboard (Next.js) → `/api/proxy/[...path]` (Next.js Route Handler) → Go API server (`/api/v1/...`)

The dashboard has no direct DB access. All data goes through the Go API. The proxy at `dashboard/src/app/api/proxy/[...path]/route.ts` forwards every request with the `valt_access_token` cookie as a Bearer token.

### Go server (`server/`)
Single binary, chi router. Entry point: `server/cmd/server/main.go`.

Internal packages (one domain per package):
- `auth` — JWT RS256, Argon2id password hashing, Google OAuth, middleware
- `vault` — secret CRUD; metadata and encrypted blobs in Postgres (secret_blobs)
- `workflow` — access request state machine, approval chain, credential issuance
- `dynsecret` — dynamic secret providers (Postgres DB credentials); AES-256-GCM encrypted at rest
- `scanner` — secret scanning results
- `rbac` — project-scoped role permissions (`owner > admin > member > viewer`)
- `ratelimit` — Redis sliding-window rate limiter (optional; gated on `X-Agent-ID` header)
- `usage` — plan limit enforcement (org-scoped counts)
- `audit` — append-only SHA-256 hash chain audit log
- `gateway` — HTTP forward proxy for credential injection; agents use `HTTPS_PROXY` + placeholder keys

Shared packages under `server/pkg/`:
- `crypto` — `EncryptAES256GCM` / `DecryptAES256GCM` (`[12-byte nonce || ciphertext+tag]`)

Config is loaded from env vars via `envconfig` in `server/internal/config/config.go`. Required: `DATABASE_URL`. Set `VAULT_MASTER_KEY` (base64 32-byte); without it an ephemeral key is used (secrets lost on restart).

### Encryption model
- **Secret values**: zero-knowledge, client-side. Client generates a DEK, encrypts with it, sends `encrypted_blob` + `encrypted_dek` (DEK wrapped by master key). Server stores blob and DEK in Postgres.
- **DynSecret provider config & lease credentials**: server-side AES-256-GCM using `masterKey` from config.
- **Master key** (`VAULT_MASTER_KEY`): base64-encoded 32-byte key; wraps DEKs for secret values and directly encrypts dynsecret data.

### Approval state machine
`pending → approved/rejected → active → expired/revoked`

`workflow/approval-chain.go` — multi-step chain; `AdvanceChain` is transactional (serializable isolation).
`workflow/handler.go` — `GetRequest`, `Approve`, `Reject` allow: the requester, assigned approvers (via `approval_steps`), or the secret owner.

### RBAC
`rbac/middleware.go` — `rbac.Middleware(db, "project_id", resource, action)` checks `project_memberships` table. Returns 400 on missing `project_id`, 403 on non-member. Resources: `secret`, `project`, `agent`, `scans`, `dynsecret`. Roles: `owner`, `admin`, `member`, `viewer`.

For routes scoped by non-project IDs (scan_id, provider_id, lease_id), handlers do inline lookups: scan → project, provider → project, lease → provider → project.

### Dashboard (`dashboard/`)
Next.js 15 App Router with `(auth)` and `(dashboard)` route groups. All authenticated pages live under `(dashboard)`. Session is read from `valt_access_token` cookie via `src/lib/auth.ts:getSession()`. No client-side token storage.

### MCP server (`mcp-server/`)
Rust binary (`valtag-mcp-server`). Runs on developer machine alongside the AI agent. Communicates via stdio JSON-RPC 2.0 (default) or HTTP. Connects to the Go API with a stored agent token. Path traversal prevention: rejects absolute paths, Windows drive letters, `..`, paths > 500 chars.

### Database migrations
Sequential numbered SQL files in `server/internal/database/migrations/`. Currently at 000046. Migration runner: `valtag-migrate` binary (`server/cmd/migrate/`).

## Code Standards
- Go: standard project layout, `internal/` for private packages
- TypeScript: strict mode, no `any`
- Rust: `cargo clippy` clean, no `unsafe` unless justified
- All: kebab-case file names, files under 200 lines
- See `docs/code-standards.md` for full details

## Key Environment Variables

| Variable | Required | Notes |
|----------|----------|-------|
| `DATABASE_URL` | Yes | Postgres connection string |
| `VAULT_MASTER_KEY` | Strongly recommended | Base64 32-byte AES key; ephemeral if unset |
| `JWT_PRIVATE_KEY_PATH` / `JWT_PUBLIC_KEY_PATH` | Yes | RS256 PEM files |
| `REDIS_URL` | No | Enables agent rate limiting (60 rpm) |
| `SMTP_HOST` | No | Email notifications; no-op if unset |
| `BACKEND_URL` | Dashboard | Default `http://localhost:8080` |
| `GATEWAY_ENABLED` | No | Enable HTTP proxy gateway (`true`/`false`) |
| `GATEWAY_PORT` | No | Gateway listen port (default `10256`) |
