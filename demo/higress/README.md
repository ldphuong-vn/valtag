# valtag × Higress — zero-credential gateway demo

A working demo of [valtag](https://github.com/ldphuong-vn/valtag) acting as an
**ask-first credential gate** in front of a [Higress](https://higress.ai) gateway:
agents never hold real upstream credentials — they hold short-lived *derived keys*
minted per lease, and Higress swaps them for the real credential on every request
by asking valtag.

## The problem this demo shows

| | "before" — static key-auth | "after" — valtag via ext-auth |
|---|---|---|
| Where the credential lives | **plaintext in the gateway config** | valtag vault (encrypted), gateway holds nothing |
| Lifecycle | never expires, manual rotation | lease TTL (minutes–hours), instant revoke |
| Granting access | anyone with the key | ask → human approves → key issued per lease |
| Audit | gateway traffic log only | every request lands in valtag's hash-chained audit trail |
| `gateway config` contains | `credential: "sk-live-…"` | only a *provider id* — zero credentials |

Run `./apply.sh --before` to see the static-key world, then `./apply.sh` to see
the valtag world, on the same gateway.

## Architecture

```
agent (valtag_dk_…) ──▶ HIGRESS ──▶ whoami (echoes received headers)
                          │  ▲
             ext-auth ────┘  └── returns the real credential as headers
                          │     (Authorization: Bearer sk-upstream-…, x-valtag-*)
                          ▼
                     VALTAG :10256 /gateway/v1/authorize
                     validate lease → resolve provider credential → audit
```

- Higress `ext-auth` plugin (forward_auth mode): `failure_mode_allow: false` —
  **fail-closed**. If valtag is unreachable, requests are rejected with 403.
- valtag returns the credential only as *response headers*, only for a live lease.
- The derived key is never forwarded upstream — it is replaced by
  `allowed_upstream_headers`.

## Quickstart (~10 minutes)

Prereqs: Docker + compose v2, bash + curl. Everything else (even JSON parsing)
runs inside the containers.

```bash
# from the repo root — 1) valtag stack
cp .env.example .env
#   edit .env:  POSTGRES_PASSWORD, SERVER_KEK, VAULT_MASTER_KEY
#   generate with: openssl rand -base64 32
make dev                                        # postgres + server + dashboard

# 2) higress demo stack (creates the shared network)
cd demo/higress
docker compose up -d

# 3) wire valtag into it (gateway listener + DNS alias)
cd ../.. && docker compose -f docker-compose.yml \
  -f demo/higress/valtag-compose.demo.yml up -d server

# 4) configure the gateway (route + plugins, no console clicking)
cd demo/higress && ./apply.sh

# 5) seed demo data + mint a lease (prints VALTAG_KEY=…)
./seed-lease.sh

# 6) the 60-second story, or the full check matrix
./demo.sh
./smoke.sh
```

`demo.sh` output looks like:

```
── 2) call the gateway with the derived key ──
   gateway → 200
   upstream received: Authorization: Bearer sk-upstream-real-credential-demo
   upstream received: X-Valtag-Lease-Id: 6f0e…
── 4) same key again ──
   gateway → 401
>>> DEMO OK: credential was live per-lease, and died the moment the lease was revoked.
```

## Files

| File | Purpose |
|---|---|
| `docker-compose.yml` | Higress all-in-one + whoami echo backend |
| `valtag-compose.demo.yml` | override: gateway listener + network alias for valtag |
| `cr/*.json` | the Kubernetes-flavored CRs `apply.sh` posts (route, registries, plugins) |
| `apply.sh` | applies CRs through Higress' internal apiserver; `--before`/`--after` |
| `seed-lease.sh` | creates org→workspace→project→provider, mints a 1h lease |
| `demo.sh` | the 60-second narrative (mint → use → revoke → 401) |
| `smoke.sh` | check matrix incl. fail-closed (`--failclosed`) |

## Production notes (things a demo deliberately skips)

- **TLS**: the all-in-one gateway ships a self-signed cert on :443. Terminate TLS
  properly (LB / reverse proxy / Cloudflare with an Origin Certificate) before
  exposing anything — derived keys travel in the `Authorization` header.
- **Console**: exposed on `127.0.0.1:8001` in this compose. Reach it via an SSH
  tunnel; the first visit initializes the admin account. Do not expose it as-is.
- **Seed credentials**: `admin@valt.dev` / `ValtAdmin2026!` is the well-known seed
  login. Change it before any non-toy use.
- **Model B**: this demo is valtag's "Model A" (zero Higress code — config only).
  The cached-credential wasm-go plugin is the follow-up; trade-offs in
  [`docs/higress-research-5w1h.md`](../../docs) of the valtag workspace.

## Troubleshooting

- **401 "invalid or expired credential"** — the agent must send the scheme:
  `Authorization: Bearer valtag_dk_…`. A bare key is ignored by the gateway listener.
- **403 with a valid key** — ext-auth could not reach valtag (fail-closed by
  design). Check `docker network` adjacency: valtag's `server` container must be
  reachable as `valtag.demo.internal` (see `valtag-compose.demo.yml`).
- **503 on /whoami** — route applied but the upstream cluster is not resolving
  yet; wait a few seconds, or check `docker exec <higress-ct> curl -s
  127.0.0.1:15000/clusters | grep whoami`.
- **Lease expired (1h TTL)** — just run `./seed-lease.sh` again; it is idempotent.
- **409 Conflict on apply** — a concurrent config change bumped
  `resourceVersion`; re-run `./apply.sh` (it re-reads the live object).
