# valtag — Domain terminology

Shared vocabulary. Use these terms exactly in code, docs, commit messages and
conversation — synonyms create drift.

## Credentials

- **Secret** — a stored credential item (name, type, encrypted value). Client-side
  encrypted: the client generates a **DEK** (data encryption key), encrypts the
  value, wraps the DEK with the master key; the server stores
  `encrypted_blob` + `encrypted_dek` and never sees plaintext.
- **VAULT_MASTER_KEY** — base64 32-byte key wrapping secret DEKs and encrypting
  dynsecret data. Loss = vault unrecoverable. Rotated with
  `valtag-rotate-master-key` (see docs/key-rotation.md).
- **Credential_type** — `api_key`, `oauth_token`, `db_credential`, `ssh_key`,
  `cloud_credential`, `personal_session`. Must match the server's
  `validCredentialTypes`.

## Access flow (ask-first)

- **Access request** — an agent or user asks for a secret. State machine:
  `pending → approved/rejected → active → expired/revoked`.
- **Policy template** — per-project reusable parameters governing access:
  `max_duration_minutes`, `require_approval`, `allow_auto_approve`,
  `single_use`, `max_requests_per_day`, `cool_down_minutes`…
  System templates (`is_system`) are immutable baselines — customize via
  **Clone** or per-secret **override_parameters**.
- **Binding** — a secret ↔ template link with optional `override_parameters`;
  the **effective policy** = template parameters merged with overrides.
- **Approver** — human who approves/rejects (dashboard, Telegram, Slack).
  The agent can never approve its own request.

## Agent credentials

- **Derived key** — `valtag_dk_<hex>` issued with an approved **lease**; bound
  to one agent, expires with the lease, dies on revoke. Verify via SHA-256
  hash lookup (`dynamic_leases.key_hash`).
- **Lease** — time-boxed grant behind a derived key (TTL ≤ 1h for minted
  leases). Revocable instantly by the owner agent or a user.
- **Agent token** — long-lived bearer token identifying an agent (issued via
  dashboard or `POST /agents/{id}/tokens`). Required alongside the derived
  key for agent-bound leases.
- **Gateway listener** — optional `POST /gateway/v1/authorize` on port
  `GATEWAY_PORT` (10256): AI gateways present `Authorization: Bearer
  valtag_dk_…` and receive the real upstream credential as response headers
  (fail-closed 401/403). Contract: docs/ax.md.

## Observability

- **Audit hash chain** — append-only event log; each entry hashes
  `hash_prev` into its own hash, making tampering detectable. Anchored
  off-site periodically (docs/key-rotation.md).
- **Agent rate limiting** — per-agent fixed window (Redis, 60 rpm) when
  `REDIS_URL` is set; unset = disabled with a loud startup warning.

## Surfaces

- **Dashboard** — Next.js operator UI. Auth cookies `valt_access_token`
  (15-minute JWT) + `valt_refresh_token` (7-day, rotated). All API calls go
  through the BFF proxy (`/api/proxy/[...path]`) which injects the Bearer
  token and single-flight-refreshes on 401.
- **Gateway listener** — the machine-facing proxy/authorize surface on
  `GATEWAY_PORT`.
- **Higress** — AI gateway integrating valtag via the ext-auth plugin
  (demo: demo/higress/, contract: docs/ax.md).
