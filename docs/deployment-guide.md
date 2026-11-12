# valtag Deployment Guide

> Rewritten 2026-11-07 (b872885). The previous guide was from the MinIO era —
> MinIO is gone: **Postgres stores everything**, including encrypted secret
> blobs (`secret_blobs`, 1 MiB cap). One database, one dump = one vault.

## 0. Components

| Component | What it does | Notes |
|---|---|---|
| `postgres` (16) | all state: orgs, secrets (encrypted blobs), leases, audit hash-chain | the only stateful service |
| `valtag-server` | API + dashboard backend; **optional gateway listener** for AI-gateway integrations | image ships `valtag-server`, `valtag-migrate`, `valtag-seed`, `valtag-rotate-master-key` |
| `dashboard` | Next.js operator UI | talks to server over the compose network |
| `caddy` | TLS / reverse proxy | dev uses `Caddyfile.ip` (plain HTTP on :8443), prod uses `Caddyfile.prod` (Let's Encrypt) |

## 1. Local development

```bash
git clone https://github.com/ldphuong-vn/valtag.git && cd valtag
cp .env.example .env
#   set SERVER_KEK and VAULT_MASTER_KEY — each: openssl rand -base64 32
./scripts/gen-keys.sh      # JWT keypair into ./keys (idempotent)
make dev                   # postgres + server + dashboard + caddy
make migrate-up            # migrations + audit_logs partitions (see note below)
make seed                  # dev data — seed passwords are PUBLIC, dev only
```

| Service | URL |
|---|---|
| API | http://localhost:8080 |
| Dashboard | http://localhost:3000 |
| Caddy proxy | http://localhost:8443 |
| PostgreSQL | localhost:5432 |

Optional — gateway listener (what an AI gateway calls, see `demo/higress/`):
add `GATEWAY_ENABLED=true` + `GATEWAY_PORT=10256` to the server environment and
publish the port **on loopback only**. Never expose it directly.

## 2. Production on a single VPS

### Prerequisites

- Ubuntu 24.04, 2 vCPU / 4 GB RAM minimum (the current staging box runs 8/8
  comfortably alongside an AI gateway)
- Docker Engine + compose v2 plugin (`apt install docker.io docker-compose-v2`)
- A domain with an A record to the VPS; 80/443 reachable
- Baseline hardening: SSH key-only (`PasswordAuthentication no`), `ufw` limited
  to 22/80/443, fail2ban with sshd aggressive + recidive jails

### Deploy

```bash
git clone https://github.com/ldphuong-vn/valtag.git && cd valtag
cp .env.example .env
#   POSTGRES_PASSWORD, SERVER_KEK, VAULT_MASTER_KEY → openssl rand -base64 32 each
#   store the master key OFF the server (password manager) — losing it loses the vault
./scripts/gen-keys.sh && chmod 600 keys/private.pem
docker compose -f docker-compose.prod.yml up -d --build
docker compose -f docker-compose.prod.yml run --rm server valtag-migrate up
docker compose -f docker-compose.prod.yml run --rm server valtag-seed   # first boot only
```

**Partitions:** `valtag-migrate up` now also creates the `audit_logs` monthly
partitions (current + next 2 months, `b872885`). Before that fix, a fresh
install rejected audit writes until the server started. The server's partition
manager keeps them topped up daily afterwards; both are idempotent.

### TLS — pick one

1. **Caddy + Let's Encrypt** (default path): in `docker-compose.prod.yml`
   switch the Caddyfile mount to `./Caddyfile.prod`, set `DOMAIN` +
   `ACME_EMAIL` in `.env`. Caddy handles issuance and renewal.
2. **Cloudflare proxied**: orange-cloud the record, set SSL mode **Full
   (strict)**, create a Cloudflare **Origin Certificate** and terminate TLS on
   your edge gateway instead of Caddy (the valtag × Higress staging does this —
   see `demo/higress/` and its origin-cert notes).

Do **not** run Caddy on 443 *and* an edge proxy on 443 at once; terminate TLS
in exactly one place.

### Environment flags that matter in prod

| Flag | Value | Why |
|---|---|---|
| `TRUST_PROXY_XFF` | `true` only when behind Caddy/edge | audit IP attribution; `false` = headers are client-controlled (report §4.7) |
| `METRICS_ENABLED` | `true`, scrape internally only | `/metrics` has **no auth** — keep it off the public interface |
| `GATEWAY_ENABLED` | `true` only if attaching an AI gateway | publish the gateway port on loopback / private network, never 0.0.0.0 |
| `SMTP_*` | real SMTP or leave empty | empty = registration auto-verifies (SMTP-aware verification, §4.15) |

### Verify

```bash
curl https://api.example.com/health          # → ok
docker compose -f docker-compose.prod.yml ps # all Up, postgres healthy
```

### Day-2 operations

- **Update**: `git pull` → `docker compose -f docker-compose.prod.yml up -d --build`
  → `... run --rm server valtag-migrate up` (idempotent, also tops up partitions).
- **Backup**: `pg_dump` of the single database = the whole vault. Verify the
  dump restores; keep copies off-site. (`scripts/restore-backup.sh`)
- **Master key rotation**: `valtag-rotate-master-key` re-wraps every encrypted
  column in one transaction — runbook in [`docs/key-rotation.md`](key-rotation.md),
  including the off-site key-storage rules.
- **Audit chain**: the audit log is hash-chained; anchor the chain head off-site
  periodically (see key-rotation doc) so tampering is detectable.

## 3. Attaching an AI gateway (Higress)

The gateway listener (`GATEWAY_PORT`, default 10256) speaks the
`POST /gateway/v1/authorize` contract: derived key in `Authorization: Bearer
valtag_dk_…` → real upstream credential + `x-valtag-*` headers in the response.
A complete, runnable integration (before/after, fail-closed test) lives in
[`demo/higress/`](../demo/higress/README.md). Wire rules:

- gateway → valtag over a private docker network or loopback, never the internet
- the gateway is the **only** client that should reach `:10256`
- key-auth (or any static credential plugin) stays **off** on routes guarded by
  valtag — the derived key replaces it

### Serving the dashboard and the API on one domain (edge = AI gateway)

When the edge proxy in front of valtag is the AI gateway itself (like the
staging deployment: `https://valt.turbo.ai.vn`), the dashboard's **Next.js route
handlers** must reach the dashboard, not the API server — otherwise login and
every dashboard page break with 404s (found in dogfood 2026-11-12):

| Path prefix | Route to | Why |
|---|---|---|
| `/api/auth/*` | **dashboard** | `set-tokens` / `refresh` / `logout` are Next.js route handlers that manage auth cookies |
| `/api/proxy/*` | **dashboard** | the dashboard's BFF proxy reads the auth cookie and forwards to the API with the token — every dashboard page loads data through it |
| `/api/*` (rest) | **valtag server** | the actual API |
| `/` | **dashboard** | |

Longest-prefix matching wins, so listing `/api/auth` and `/api/proxy` before
`/api` is enough. Also remember: secrets are **project-scoped** — the create
endpoint rejects `project_id`-less payloads, and the dashboard binds the
current project from its workspace context (which self-heals in the sidebar on
returning logins).
