# Working on valtag

valtag is an MCP-native secret vault with human-in-the-loop approval for AI
agents. Monorepo: Go backend (`server/`), Next.js dashboard (`dashboard/`),
Rust MCP server (`mcp-server/`). Self-hosted: Postgres is the only stateful
service; secrets are client-side encrypted (DEK + master key).

`CLAUDE.md` is a pointer to this file; always edit `AGENTS.md`.

## Progressive context loading

Start with this file and the task's source files. Read only the guide relevant
to the task — executable code owns field lists, defaults and schemas; the docs
below explain system boundaries, rationale and operator procedures:

| Task | Read |
| --- | --- |
| Domain terminology (lease, DEK, derived key…) | [CONTEXT.md](CONTEXT.md) |
| Architecture or security boundaries | [docs/system-architecture.md](docs/system-architecture.md), [docs/security-model.md](docs/security-model.md) |
| API, gateway or authorize contract | [docs/ax.md](docs/ax.md) |
| Deployment (dev, VPS, domain + AI gateway) | [docs/deployment-guide.md](docs/deployment-guide.md) |
| Key rotation, master key loss, backups | [docs/key-rotation.md](docs/key-rotation.md) |
| Code standards or PR expectations | [docs/code-standards.md](docs/code-standards.md) |
| Install with a coding agent | [docs/install-for-agents.md](docs/install-for-agents.md) |
| Higress integration demo | [demo/higress/README.md](demo/higress/README.md) |

Do not read every guide before editing. When a claim in docs contradicts the
code, trust the code and fix the doc in the same change.

## Commands

```bash
# Full dev environment (Docker required)
make dev              # Start all services (server, dashboard, postgres, caddy)
make down             # Stop all services
make migrate-up       # Migrations + audit_logs partitions (idempotent)
make seed             # Seed dev data — seed passwords are PUBLIC, dev only

# Testing
make test             # All tests (Go unit + dashboard + Rust)
make test-unit        # Go unit tests only (no DB)
make test-integration # Go integration tests (requires Docker)
make test-dashboard   # Dashboard: cd dashboard && npm test
make test-mcp         # Rust: cd mcp-server && cargo test

# Run a single test
cd server && go test ./internal/workflow/... -run TestApproveRequest -v
cd dashboard && npx vitest run src/path/to/test.ts

# Linting
make lint             # All linters
cd server && golangci-lint run ./...
cd dashboard && npm run lint
cd mcp-server && cargo clippy -- -D warnings
```

## Judgment and autonomy

- Trust only code — every claim carries `file:line` evidence. When docs
  contradict code, fix the doc in the same change.
- Secrets and keys never appear in tool output: no `.env` contents, no
  `VAULT_MASTER_KEY`, no decrypted values, no `docker compose config`. The
  audit chain and credential values are read-only data — never print them.
- Before adding complexity for a speculative edge case, explain the specific
  failure mode, its likelihood and the complexity cost; obtain approval before
  expanding scope.
- Migrations are append-only sequential SQL (`server/internal/database/migrations/`)
  with the runner in `server/cmd/migrate/` — `migrate-up` also ensures audit
  partitions. Never edit an applied migration; add a new one and test it.
- Changing auth or crypto behaviour (JWT claims, hashing, encryption, token
  versions) requires an integration test in the same change.
- Renaming or moving user-facing surfaces (dashboard routes, sidebar entries)
  keeps deep links working — check for inbound links before removing a route.

## Architecture in one screen

```text
Dashboard (Next.js) ──▶ /api/proxy/[...path] (BFF, cookie→Bearer)
   ├─▶ Go API server (chi router, /api/v1/...)
   │      ├─ vault (secret CRUD, client-side DEK encryption)
   │      ├─ workflow (approval state machine, credentials)
   │      ├─ dynsecret (dynamic providers, derived keys valtag_dk_)
   │      ├─ audit (append-only SHA-256 hash chain)
   │      └─ policy (templates, bindings, overrides)
   ├─▶ Postgres (only stateful service)
   └─▶ gateway listener :10256 (optional; POST /gateway/v1/authorize
         for AI gateways — ext-auth contract, fail-closed)
MCP server (Rust, stdio/HTTP) ──▶ Go API with agent token
Higress/AI gateway ──▶ valtag authorize ──▶ inject credential per request
```

Per-package details, the encryption model, the approval state machine and RBAC
roles live in [docs/system-architecture.md](docs/system-architecture.md) — read
that before changing cross-package boundaries.

## Key environment variables

| Variable | Required | Notes |
|----------|----------|-------|
| `DATABASE_URL` | Yes | Postgres connection string |
| `VAULT_MASTER_KEY` | Recommended | Base64 32-byte key; ephemeral if unset |
| `JWT_PRIVATE_KEY_PATH` / `JWT_PUBLIC_KEY_PATH` | Yes | RS256 PEM files |
| `REDIS_URL` | No | Agent rate limiting (60 rpm); unset = disabled with a loud warning |
| `GATEWAY_ENABLED` / `GATEWAY_PORT` | No | Gateway listener (default `10256`) |
| `SMTP_HOST` | No | Email notifications; no-op if unset |
