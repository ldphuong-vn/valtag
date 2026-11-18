# valtag × LiteLLM — ask-first gate for the LLM proxy

[LiteLLM](https://github.com/BerriAI/litellm) proxies 100+ LLM providers with
virtual keys, budgets and teams. With the valtag gate, **every LLM call
requires a valtag-approved lease** — LiteLLM's own master key does not bypass
it (verified live).

## How it works

```text
Agent/user ──▶ LiteLLM (:14000) ──▶ valtag_gate.py (custom auth)
                    │                    └─ POST valtag /gateway/v1/authorize
                    │                        Authorization: Bearer valtag_dk_…
                    │                        ◀── 200 (lease valid) / 401 / 503
                    ▼
               LLM provider (per LiteLLM model_list)
```

- The gate replaces LiteLLM's key auth entirely (`general_settings.custom_auth`)
  — LiteLLM's own master key does **not** bypass it (verified live: it gets
  401 from the gate).
- Fail-closed: valtag unreachable → 503.
- The audit chain records every gated call.

## Files

| File | Purpose |
|---|---|
| `valtag_gate.py` | the custom auth callback (fail-closed, agent-id aware) |
| `config.yaml` | LiteLLM config: mock model + an `oidc/file` upstream-key model |
| `docker-compose.yml` | LiteLLM wired to the valtag gateway listener |
| `oidc_file_sidecar.sh` | lease sidecar for the upstream-key recipe (see below) |

## Route coverage (verified live, litellm 1.109.1)

`general_settings.custom_auth` runs on the API surface — including routes you
might not expect:

| Route | No key | Valid lease | Master key |
|---|---|---|---|
| `POST /v1/chat/completions` | ✅ 401 | ✅ 200 | ✅ 401 |
| `GET /v1/models` | ✅ 401 | ✅ 200 | ✅ 401 |
| `POST /v1/embeddings` | ✅ 401 | ✅ 200 | — |
| `GET /health/liveliness`, `/health/readiness`, `/` (UI) | public — 200, no auth | | |

With no `Authorization` header LiteLLM passes `api_key=None` into the
callback; the gate denies cleanly (`401 valtag denied: missing credential`)
instead of crashing on it.

## Upstream key — leases instead of long-lived provider keys

The gate protects the **inbound** side. For the **outbound** side LiteLLM
supports file-based upstream keys that are **re-read on every call** (since
v1.109.0): set the deployment's `api_key` to

```yaml
api_key: "oidc/file//run/secrets/valtag-lease"
```

and run the sidecar (`oidc_file_sidecar.sh`) next to LiteLLM — it mints a
lease from valtag and atomically replaces the file before each expiry. The
provider call then carries `Authorization: Bearer <current lease value>`.

**What we verified live** (staging, litellm 1.109.1, a fake OpenAI upstream
logging the received `Authorization` header):

| Step | Upstream received |
|---|---|
| call 1 (file = lease-1) | ✅ `Bearer valtag_dk_5528…` |
| sidecar mints lease-2, call 2 | ✅ `Bearer valtag_dk_f010…` — **the new value, no cache** |
| file overwritten with a dead key, call 3 | ✅ `Bearer valtag_dk_dead…` — passed through verbatim |

An empty/missing file makes LiteLLM fail the request
(`AuthenticationError`) — fail-closed, not silent.

**Limits, stated plainly:**

- This is **one shared lease** for the whole proxy — every LiteLLM caller uses
  the same upstream credential. Per-caller isolation is the inbound gate's
  job, not this sidecar's.
- Whether the rotated lease is *enforced* depends on the provider rejecting
  stale keys — our test upstream accepts anything, so enforcement was not
  part of this check.
- The sidecar needs an agent token with lease-mint rights; store it like any
  other secret (it can mint leases, not read secrets).

## Deploy

Prereqs: valtag running with `GATEWAY_ENABLED=true` (listener on
`127.0.0.1:10256`), an agent created and an agent token issued (Dashboard →
Agents), and the `higress-demo`-style gateway demo is optional — this
integration talks to valtag directly.

```bash
mkdir -p /opt/litellm-valtag && cd /opt/litellm-valtag
# copy integrations/litellm/{valtag_gate.py,config.yaml,docker-compose.yml}
printf 'LITELLM_MASTER_KEY=%s\n' "$(openssl rand -hex 24)" > .env   # startup formality
docker compose up -d
```

Swap the mock model in `config.yaml` for real providers, then:

```bash
# mint a lease (or let an agent request one via MCP)
curl -X POST http://127.0.0.1:8080/api/v1/providers/<provider_id>/leases \
  -H "Authorization: Bearer <agent-token>" \
  -d '{"agent_id":"<agent_id>","ttl_seconds":600}'

# call LiteLLM with the derived key → 200 + completion
curl http://127.0.0.1:14000/v1/chat/completions \
  -H "Authorization: Bearer valtag_dk_…" \
  -d '{"model":"valtag-mock","messages":[…]}'

# same key after revoke → 401
```

## Security notes

- **The raw lease key never reaches LiteLLM's logs.** LiteLLM hashes
  `UserAPIKeyAuth.api_key` only when it starts with `sk-`; anything else is
  copied verbatim into `user_api_key_hash` (logs, spend logs, Langfuse /
  Datadog / OTEL payloads). The gate therefore hands LiteLLM a one-way
  fingerprint (`valtag-<sha256[:24]>`): per-lease attribution survives, the
  credential does not leak.
- Gate error logs carry the exception type only — never a traceback, which
  can echo request headers.
- Keep the valtag authorize URL on loopback/private networking; the gate sends
  the lease key to it on every request.

## Verified matrix (staging, litellm:main-stable, 1.109.1)

| Request | Result |
|---|---|
| `Authorization: Bearer valtag_dk_…` (valid lease) | ✅ 200 + completion |
| no key | ✅ 401 (clean denial — `api_key=None` handled) |
| dead key | ✅ 401 |
| **LiteLLM master key** | ✅ **401 — the gate denies even the proxy's own admin key** |
| valtag stopped | ✅ 503 fail-closed |
| raw lease key in container logs (full history) | ✅ 0 occurrences (fingerprint instead) |
| upstream key rotation via `oidc/file` sidecar | ✅ re-read every call (see above) |
