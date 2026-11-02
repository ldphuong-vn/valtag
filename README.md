# valtag — the ask-first credential gate for AI agents

[![CI/CD](https://github.com/ldphuong-vn/valtag/actions/workflows/ci.yml/badge.svg)](https://github.com/ldphuong-vn/valtag/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

Your AI agent needs an API key. Today you paste a long-lived production key into its config — and it stays there, forever, on every machine the agent runs on, with no record of what the agent did with it.

**valtag flips that.** It is a self-hosted gate that sits between the agents you *already have* and your credentials: agents *request* access, a human *approves*, the agent receives a **short-lived lease** that auto-expires, and every step lands in a **tamper-evident audit trail**. Like issuing a laptop to an employee — except for AI credentials.

```
Agent ──"I need this key for 30 min"──▶ valtag ──▶ Human approves (dashboard / Slack / email)
  ▲                                       │
  └────── short-lived lease ◀─────────────┘
              every request audited, revocable in one command
```

**No new harness required.** valtag is not an agent platform — you don't move your agents into a sandbox or a runner. Keep Claude Code, Cursor, ZCode, your CI scripts and cron jobs exactly where they are. Attach one MCP server + one URL, and every credential they touch goes through the gate.

## Who is it for

- **Solo founders** shipping AI features: give your agents real credentials without leaving production keys scattered in config files and `.env` copies.
- **IT / platform teams**: onboarding an employee to AI tooling becomes provisioning — an agent identity, a policy, short leases. Offboarding is one command that kills every lease and agent token they hold.
- **Dev teams** building agent features: your agents talk to valtag through standard MCP tools (`request_access` → `get_credential`) instead of you building secret handling into each agent.

## Why valtag (and not the alternatives)

| | Static secret store | Agent platforms (harness + gateway) | Silent brokering (proxy attaches creds) | valtag |
|---|---|---|---|---|
| What the agent holds | The real, long-lived key | — (agent lives in their sandbox) | Nothing, by design | A lease that expires in minutes-to-hours |
| Human in the loop | No | Approval *inside their harness* | No (or one-step consent) | Per-request approval, policy-gated (reason, duration, cooldown, daily limits) — as a standalone service |
| Works with your existing agents | Yes but unsafe | Requires adopting their platform | Yes | Yes — any MCP client, CLI, or script |
| Dynamic credentials | No | Not published | Not published | Mints throwaway creds per approval: temp Postgres roles (`VALID UNTIL`), derived API keys (`valtag_dk_…`) |
| Audit | Who read the secret, maybe | Vendor log | Vendor log | Who asked, why, where from (IP + UA), who approved — SHA-256 hash chain you can verify, with off-site anchoring |
| Self-host | Varies | Varies (core Apache-2.0, `ee/` gated) | Commercial | Pure Apache-2.0, one Go binary + Postgres |

For **dynamic backends** valtag goes further: it mints credentials that never existed before the approval — a temporary Postgres role, or a derived API key HMAC'd from your master key, which no upstream API accepts and only valtag's gateway understands. Your real key never leaves the vault.

## How it works

```
                        ┌── dashboard / Slack / email ──▶ Human (approves)
                        │
ZCode · Claude Code ·   │ 1. request_access(secret, reason, ttl)
Cursor ──MCP/stdio──▶ valtag-mcp-server ──HTTPS──▶ valtag server
                        │ 2. check_status → approved          │  approval → lease minted
                        └─ 3. get_credential ────────────────┘  audit: who/what/when/where/why/how
```

One server, three ways to consume credentials:

1. **MCP tools** (agents): `request_access` → human approves → `get_credential`. Works with any MCP client — Claude Code, Claude Desktop, Cursor, ZCode.
2. **CLI injection** (humans & scripts): `valt run -- ./deploy.sh` runs your command with approved secrets injected as environment variables.
3. **Gateway proxy** (existing tooling): point tools at the proxy with a `valtag_pk_` placeholder; the real credential is injected inline on the way out.

## Quick start

**1. Run the server** (Docker, self-hosted — no cloud dependency):

```bash
git clone https://github.com/ldphuong-vn/valtag.git
cd vaultsaas
cp .env.example .env
docker compose up -d
make migrate-up
# API: http://localhost:8080 · Dashboard: http://localhost:3000
```

**2. Set up your machine** (one-time; installs the two clients):

```bash
valt setup                    # login, stores token in your OS keychain
valt mcp install --ide claude # wires valtag-mcp-server into Claude Code / Cursor / VS Code
```

**3. First request** — from your AI tool, or from a terminal:

```
Agent: request_access(secret_id, reason="generate weekly report", duration_minutes=30)
You:   approve on Slack / dashboard
Agent: get_credential → usable for 30 min, then gone
```

```bash
valt request prod-db-api       # same flow, human-typed
valt run -- ./report.sh        # approved secrets injected as env vars
```

**4. Make a secret dynamically leased** — link it to a provider, and approvals mint fresh credentials instead of handing out the stored value:

```bash
# create a derived_api_key provider (your master key never leaves valtag),
# then link: PUT /secrets/{id}/dynamic-provider  {"provider_id": "..."}
```

## What's inside

| Component | Tech | What it does |
|---|---|---|
| `server/` | Go | Vault (envelope encryption), approval workflow + policy engine, dynamic lease providers, gateway proxy, audit hash-chain |
| `dashboard/` | Next.js | Approvals, secrets, agents, audit review |
| `mcp-server/` | Rust | The local MCP bridge agents talk to (stdio or HTTP transport) |
| `valt-cli/` | Go | The single entry point: setup, IDE wiring, request/status/run |
| `sdk/python` | Python | Client SDK for custom integrations |

### Feature status

Shipped (integration tests run against a real Postgres when `DATABASE_URL` is provided; otherwise they skip):

- **Dynamic leases** — Postgres temporary roles (auto-expiring, swept clean after expiry) and derived API keys (HMAC-based, validated by hash, swapped for the master key at the gateway)
- **Approval workflow** — dashboard, Slack (HMAC-verified interactions), email action links; policy engine per secret (max duration, reason required, cooldowns, daily limits, auto-approve tiers)
- **Revoke cascade** — one command kills all leases + credential sessions + agent tokens of a user; org member removal cascades project access
- **Audit 4W1H** — every event with IP/user-agent context, SHA-256 hash chain (covers all fields, survives restarts, concurrency-safe), `GET /audit/verify` with `?expected_head=` off-site anchoring, CSV export with formula-injection guard
- **Secret scanning** — local regex scans for leaked credentials via the MCP `scan_secrets` tool, findings persisted server-side

On the roadmap toward v1.0: hardening sweep before the first tagged release (authz audit, key rotation, prod TLS topology — see `docs/positioning.md`), cloud STS lease providers, SSH certificate issuance, signed multi-platform releases (CLI pipeline ships 3 platforms; MCP server pipeline ships 6 targets).

## Security model, honestly

- Secrets are encrypted at rest (AES-256-GCM envelope encryption; blobs in Postgres, DEKs wrapped with the master key). **The server does decrypt** approved secrets to deliver them — valtag is not zero-knowledge; it is *least-knowledge*: the standing state is always ciphertext, plaintext exists only at approved delivery time.
- Credentials issued to agents are **leases with TTLs**; for derived keys the master key never leaves the server and the issued key is useless to anything but valtag's gateway.
- The audit log is a **hash chain** — editing any row (IP, metadata, timestamps) breaks verification. Verify against an off-site anchor hash to also detect tail truncation.
- Agent tokens live in the OS keychain, not in agent configs. On headless Linux, fall back to environment variables in a protected service unit.
- Self-hosted only. Your approval flow, your keys, your audit data — nothing leaves your infrastructure. Pure Apache-2.0, no `ee/` directory, no feature gates, no CLA.

> **Status: v0.x, pre-release.** Solo-maintained, moving fast. If you deploy it, read [SECURITY.md](SECURITY.md) first, change every default credential, and keep audit-chain heads anchored off-site. See [docs/positioning.md](docs/positioning.md) for the product thesis and known gaps.

See [docs/security-model.md](docs/security-model.md) for details.

## Documentation

- [Positioning & product thesis](docs/positioning.md) — USP, competitive map, what we will/won't build
- [Security model](docs/security-model.md) — encryption, auth, audit chain
- [System architecture](docs/system-architecture.md)
- [Deployment guide](docs/deployment-guide.md) — dev + single-VPS production
- [Decision log](docs/decisions.md) — why valtag is what it is (and isn't)
- [Refactor plan](docs/refactor-plan.md) — living roadmap with change logs

## Development

```bash
make help    # all commands
make dev     # dev environment
make test    # all tests
make lint    # linters
make build   # build services
```

## License

Apache License 2.0 — see [LICENSE](LICENSE). Contributions welcome; by contributing you agree your contributions are licensed under the same terms.
