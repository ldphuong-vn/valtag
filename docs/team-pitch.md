# valtag — Team Pitch

> MCP-native secret vault with human-in-the-loop approval for AI agents.

---

## The Problem

AI agents (Claude, Cursor, GPT, etc.) need credentials to do real work — API keys, database passwords, SSH keys. Today developers either:

- **Hardcode secrets in `.env`** — dangerous, leaks in logs, no visibility
- **Paste credentials into chat** — plaintext, no audit, no expiry
- **Block agents entirely** — safe but defeats the purpose

There's no middle ground: controlled, audited, time-limited access with a human deciding when an agent gets a secret.

---

## What valtag Does

1. You store secrets encrypted end-to-end (server never sees plaintext)
2. AI agents request access via the MCP Protocol
3. You approve or reject from the dashboard
4. Agent gets a temporary credential that auto-expires
5. Everything is logged in a tamper-evident audit trail

---

## Who It's For

- Dev teams (5–50 people) using AI coding assistants
- Security teams that need visibility and control over AI access
- Any team where AI agents touch production systems

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────┐
│  Developer Machine                                           │
│  ├── IDE / AI Agent (Claude, Cursor, etc.)                   │
│  │     └── valtag MCP Server (Rust, stdio JSON-RPC 2.0)       │
│  └── valtag CLI (Go) — headless secret access + env injection  │
└─────────────────────────┬────────────────────────────────────┘
                          │  HTTPS
┌─────────────────────────▼────────────────────────────────────┐
│  Go API Server                                               │
│  ├── Vault         (secret storage, envelope encryption)     │
│  ├── Workflow      (approval state machine)                  │
│  ├── Auth          (JWT RS256 + Google OAuth)                │
│  ├── RBAC          (org → workspace → project → roles)      │
│  ├── DynSecret     (dynamic DB credentials)                  │
│  ├── Notify        (Email / Slack / Telegram approval push)  │
│  ├── Policy        (3-level custom policy resolver)          │
│  └── Audit         (SHA-256 tamper-evident hash chain)       │
└──────────┬──────────────────────────┬────────────────────────┘
           │                          │
    ┌──────▼──────┐            ┌──────▼──────┐
    │  PostgreSQL  │            │    MinIO    │
    │  metadata +  │            │  encrypted  │
    │  audit logs  │            │    blobs    │
    └─────────────┘            └─────────────┘

┌──────────────────────────────────────────────────────────────┐
│  Next.js Dashboard (what humans use)                         │
│  ├── /secrets      — manage your vault + per-secret policy   │
│  ├── /approvals    — approve / reject agent requests         │
│  ├── /agents       — manage agent identities & tokens        │
│  ├── /providers    — dynamic database credential pools       │
│  ├── /scans        — detect hardcoded secrets in code        │
│  ├── /audit        — full tamper-evident audit log           │
│  └── /settings     — notification channels (Slack/Telegram)  │
└──────────────────────────────────────────────────────────────┘
```

**Tech stack:** Go 1.22 (API) · Next.js 15 / shadcn/ui (dashboard) · Rust (MCP server) · Go (valtag CLI) · PostgreSQL 16 · MinIO (S3-compatible) · Caddy (TLS proxy)

---

## Encryption — Zero Knowledge

The server stores secrets but **cannot read them**:

```
Browser / client side:
  1. Generate a random DEK (Data Encryption Key)
  2. Encrypt the secret value with the DEK  ──────▶  store in MinIO (blob)
  3. Encrypt the DEK with the master key    ──────▶  store in Postgres (metadata)

Server holds: a locked box + a locked key.
The master key lives only in your environment (VAULT_MASTER_KEY env var).
```

This mirrors the same model used by AWS Secrets Manager and HashiCorp Vault.

---

## Complete Workflow — All Cases

### Case 1: Developer stores a secret (Dashboard)

```
1. Login via Google OAuth → lands on /secrets
2. New Secret → enter name, type (api_key / db_credential / ssh_key / etc.)
3. Enter secret value → browser encrypts locally before sending
4. Encrypted blob stored in MinIO, encrypted DEK stored in Postgres
5. Plaintext never leaves the browser
```

---

### Case 2: AI agent requests access (MCP / IDE flow)

**One-time setup:**
```
1. Dashboard → /agents → Create Agent → copy token
2. Install valtag-mcp-server on dev machine
3. Set VALTAG_AGENT_TOKEN=<token>  (or use the authenticate_agent MCP tool)
4. Add to Claude / Cursor MCP config:
     { "command": "valtag-mcp-server", "transport": "stdio" }
```

**Live workflow:**
```
① Agent discovers available secrets
     Claude calls:  list_my_secrets
     valtag returns:  [ { id: "abc", name: "Stripe API Key", type: "api_key" } ]

② Agent requests access
     Claude calls:  request_secret_access
                    { secret_id: "abc", reason: "calling Stripe for payment task", duration: 60 }
     valtag creates:  access request → status: "pending"
                    returns request_id: "req_xyz"

③ Human sees request in dashboard /approvals
     ┌─────────────────────────────────────────────────┐
     │  ⏳ Agent "claude-dev" wants "Stripe API Key"   │
     │  Reason: "calling Stripe for payment task"       │
     │  Duration: 60 minutes    Type: api_key (Tier 1)  │
     │  [ Approve ]  [ Reject ]                         │
     └─────────────────────────────────────────────────┘

④ Agent polls while waiting
     Claude calls:  check_approval_status { request_id: "req_xyz" }
     valtag returns:  { status: "pending" }   ← retries every few seconds

⑤ Human clicks Approve
     valtag:  status → "approved" → "active"
            issues temporary credential (expires in 60 min)

⑥ Agent retrieves credential
     Claude calls:  get_credential { request_id: "req_xyz" }
     valtag returns:  { value: "sk_live_...", expires_in: 3600 }
     Claude uses the credential, completes the task

⑦ Credential expires automatically after 60 minutes
     OR agent calls revoke_credential when done early
```

---

### Case 3: Human rejects a request

```
③ (alternate) Human clicks Reject
   Fills rejection reason: "This task doesn't need Stripe access"

④ Agent polls and receives:
   { status: "rejected", reason: "This task doesn't need Stripe access" }

   Claude tells the user the request was blocked and why.
```

---

### Case 4: Multi-step approval chain (high-risk secrets)

For sensitive credential types (cloud credentials, personal sessions):

```
Admin configures: secret requires 2 approvers in sequence
  → Step 1: team lead
  → Step 2: security officer

Agent requests access
  → Step 1 becomes active; Step 2 is locked until Step 1 passes
  → Team lead approves → Step 2 activates
  → Security officer approves → credential is issued
  → Either approver can reject → chain stops immediately
```

The rejection reason is stored and visible to the requester.

---

### Case 5: Dynamic database credentials

Instead of a static shared password, valtag generates a fresh DB user per-session:

```
1. Admin sets up a provider in /providers:
      { type: "postgres", host: "db.prod", admin credentials }
   Provider config is encrypted with AES-256-GCM at rest.

2. Agent requests access → human approves → valtag:
   → Connects to the target database as admin
   → CREATE USER session_<uuid> WITH PASSWORD '<random 32-char>'
   → GRANT SELECT ON schema.* TO session_<uuid>
   → Returns { username: "session_abc", password: "..." }

3. Credential expires / is revoked → valtag:
   → DROP USER session_abc  ← fully removed from the database
   Static password compromise is impossible — there is no static password.
```

---

### Case 6: Secret scanning

```
From the IDE (via MCP tool):
   Claude calls:  scan_secrets { path: "./src" }
   valtag scans:    all files in ./src for hardcoded credentials
   Returns:       list of findings with file + line number

From the dashboard /scans:
   View scan history, mark findings as dismissed or imported
   Imported findings can be promoted into the vault directly
```

---

### Case 7: Approval notifications (Slack / Telegram / Email)

When an agent creates an access request, valtag pushes a notification to the approver's configured channels:

```
Slack (Block Kit message):
  ┌──────────────────────────────────────────────────┐
  │ 🔐 Access Request — "Stripe API Key"             │
  │ Agent: claude-dev   Reason: payment task          │
  │ Duration: 60 min    Tier: 2 (requires approval)   │
  │ [✅ Approve]  [❌ Reject]                         │
  └──────────────────────────────────────────────────┘
  → Clicking a button calls the valtag API directly.
  → No dashboard visit required.

Telegram: same message as inline keyboard buttons.

Email: HTML email with one-click action links (72h TTL).
  → GET /api/v1/action-tokens/{token}/redeem
  → Approver clicks link → request approved/rejected.
  → No login required.
```

Approvers configure channels in `/settings → Notifications`.

---

### Case 8: valtag CLI (headless / CI access)

For developers and scripts that don't use an IDE:

```bash
# One-time setup: OAuth login, org/project selection, agent token
valtag setup

# Install MCP server config for your IDE
valtag mcp install --ide claude   # or cursor, vscode

# List accessible secrets
valtag list

# Get a secret value (auto-requests access + polls if needed)
valtag get STRIPE_API_KEY

# Inject all project secrets as env vars and run a command
valtag run -- npm run deploy

# Create an access request manually
valtag request STRIPE_API_KEY --reason "deploy task" --duration 2h

# Check status
valtag status req_xyz
```

Works headless — useful in CI pipelines where no browser is available. Agent token stored in OS keychain.

---

### Case 9: Audit log

Every action is recorded in a tamper-evident hash chain:

```
2026-03-17 14:32:01  Agent "claude-dev"  requested  "Stripe API Key"
2026-03-17 14:32:45  User "alice@co"     approved   req_xyz
2026-03-17 14:32:45  Credential issued              expires 15:32:45
2026-03-17 15:32:45  Credential expired             req_xyz

Each entry contains SHA-256(previous entry hash + current event).
Tampering with any record breaks the chain — detectable by anyone.
```

Viewable in dashboard `/audit`, filterable by type and date.

---

## Policy Tiers — Automatic Enforcement

The system classifies secrets into tiers and enforces limits automatically:

| Tier | Secret types | Auto-approve | Max duration | Daily limit | Cool-down |
|------|-------------|-------------|--------------|-------------|-----------|
| 1 | `api_key` | ✅ Yes | 24 hours | 100/day | None |
| 2 | `db_credential`, `ssh_key`, `oauth_token` | ❌ No | 8 hours | 20/day | 1 hour |
| 3 | `cloud_credential`, `personal_session` | ❌ No | 1 hour | 5/day | 4 hours |

Tier 3 also requires a minimum 50-character reason.

---

## RBAC — Who Can Do What

```
Organization
  └── Workspace
        └── Project  ← roles are assigned here
              ├── owner / admin  → full access + can approve secrets
              ├── member         → read/write secrets, read-only on providers
              └── viewer         → read-only on everything
```

Agent rate limiting: 60 requests/minute per agent token (Redis-backed sliding window).

---

## Security Properties

| Threat | How valtag handles it |
|--------|---------------------|
| Compromised AI agent | Human approval required for every access |
| Database breach | Zero-knowledge: encrypted blobs only, no plaintext |
| Insider threat / DBA snooping | Admin cannot decrypt — they hold no master key |
| Replay attack | Credentials auto-expire; single-use option for Tier 3 |
| Brute force | Argon2id hashing + rate limiting (5 attempts/min per IP) |
| Path traversal (MCP scanner) | Absolute paths, `..`, drive letters, paths > 500 chars rejected |
| Agent token abuse | 60 rpm rate limit + token revocation from dashboard |

---

## What's Working Now

| Feature | Status |
|---------|--------|
| Secret CRUD + zero-knowledge encryption | ✅ Done |
| E2E credential delivery (decrypt + deliver to agent) | ✅ Done |
| Approval workflow (full state machine) | ✅ Done |
| Multi-step approval chains | ✅ Done |
| Temporary credentials with TTL + auto-expiry | ✅ Done |
| Dynamic DB credentials (Postgres provider) | ✅ Done |
| MCP server — 5 tools, 3 resources | ✅ Done |
| Google OAuth + JWT auth | ✅ Done |
| Org → Workspace → Project hierarchy | ✅ Done |
| Agent identity management + token rotation | ✅ Done |
| Agent cross-user access requests | ✅ Done |
| RBAC on all routes | ✅ Done |
| Agent rate limiting (Redis, 60rpm) | ✅ Done |
| Secret scanning (local files via MCP) | ✅ Done |
| Tamper-evident audit log (hash chain) | ✅ Done |
| Policy tier enforcement | ✅ Done |
| Custom policies (per-secret + per-project) | ✅ Done |
| Slack bot approvals (Block Kit + HMAC) | ✅ Done |
| Telegram bot approvals (inline keyboard + deep-link) | ✅ Done |
| Email one-click approve/reject links | ✅ Done |
| valtag CLI (setup, list, get, run, request, status) | ✅ Done |
| goreleaser cross-platform builds + CI release | ✅ Done |
| Tests (Go unit + dashboard + Rust) | ✅ Done |

---

## What's Next (Phase 3)

| Feature | Priority |
|---------|----------|
| Plan limits UI (free vs. paid tier enforcement in dashboard) | P0 |
| More dynamic providers (AWS IAM, GitHub tokens) | P1 |
| ListPending shows requests to assigned approvers (not just secret owners) | P1 |
| Key rotation mechanism for master key | P1 |
| SSO / OIDC login (enterprise) | P2 |
| Kubernetes deployment + multi-region | P2 |
| More MCP tools (scan_secrets, bulk operations) | P2 |

---

## Running It Locally

```bash
git clone <repo>
cd valtag
cp .env.example .env
# Edit .env: set VAULT_MASTER_KEY, Google OAuth credentials

docker compose up -d    # starts API, dashboard, Postgres, MinIO, Caddy
make migrate-up         # run 24 migrations
make seed               # seed dev data

# Access:
# Dashboard:  http://localhost:3000
# API:        http://localhost:8080
# MinIO UI:   http://localhost:9001
```

Generate a master key:
```bash
openssl rand -base64 32   # paste into VAULT_MASTER_KEY in .env
```
