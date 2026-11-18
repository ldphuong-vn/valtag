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
| `config.yaml` | LiteLLM config with a mock model for testing |
| `docker-compose.yml` | LiteLLM wired to the valtag gateway listener |

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

## Verified matrix (staging, litellm:main-stable)

| Request | Result |
|---|---|
| `Authorization: Bearer valtag_dk_…` (valid lease) | ✅ 200 + completion |
| no key | ✅ 401 |
| dead key | ✅ 401 |
| **LiteLLM master key** | ✅ **401 — the gate denies even the proxy's own admin key** |
| valtag stopped | ✅ 503 fail-closed |
