# Security Model

> Last checked against the code: 2026-11-18. Every claim below names the file
> it comes from so you can verify it. valtag is **least-knowledge, not
> zero-knowledge**: the server holds the master key and decrypts secrets to
> deliver them. An earlier version of this page claimed otherwise; that was
> wrong and has been corrected.

## Encryption

### Data at rest

- Algorithm: AES-256-GCM, envelope encryption.
- **Default path** (dashboard, CLI, API `value`): the **server** generates a
  random DEK per secret, encrypts the value with it, and wraps the DEK with
  the master key (`server/internal/vault/handler.go`, create path). The
  ciphertext blob goes to the Postgres `secret_blobs` table (capped at
  1 MiB); the wrapped DEK is stored with the secret row.
- **Master key**: a 32-byte key read from the server's `VAULT_MASTER_KEY`
  environment variable (`server/internal/config/config.go`). It is not
  derived from any password. If unset, the server generates an ephemeral key
  and secrets do not survive a restart. The same key protects other
  encrypted columns (TOTP secrets, provider configs, lease credentials,
  integration tokens); `valtag-rotate-master-key` re-wraps all of them in one
  transaction ([key-rotation runbook](key-rotation.md)).
- **Alternate path**: the API also accepts a client-encrypted blob plus a
  wrapped DEK (`encrypted_blob` / `encrypted_dek`), kept for backward
  compatibility and future client-side encryption (see the handler comment).
- **Consequence**: anyone who holds the database **and** the master key — or
  who can execute code on the server — can read secrets. A database dump
  alone yields ciphertext only. Plaintext exists in server memory while a
  secret is created, revealed or delivered.

### Data in transit

- TLS terminates at your edge (Caddy, Cloudflare, or your gateway) — see the
  [deployment guide](deployment-guide.md).
- JWT access tokens are signed RS256 (`server/internal/auth/jwt.go`): 15
  minutes. Refresh tokens: 7 days (`auth/handler.go:342`,
  `time.Now().Add(7*24*time.Hour)`), rotated, stored as hashes
  (`refresh_tokens.token_hash`).

### Key storage

- Client (CLI / MCP server): agent tokens in the OS keychain; on headless
  Linux, environment variables in a protected service unit.
- Agent tokens on the server: stored as SHA-256 hashes.
- Server: `VAULT_MASTER_KEY` in the environment. KMS / HSM integration is not
  implemented.

## Authentication

- Email + password; passwords hashed with Argon2id
  (`server/internal/auth/password.go`). Argon2id is used for password hashing
  only — it does not derive any encryption key.
- Login lockout: 15-minute window (`server/internal/auth/login_lockout.go`).
- Changing a password bumps `token_version` and deletes refresh tokens.

## Authorization

- Secrets belong to a project; callers must be project members.
- Access is **policy-gated**: per-secret policies set reason requirements,
  maximum duration, cooldowns and daily limits. A secret with **no bound
  policy** falls under the default auto-approve tier (observed on staging) —
  bind a policy to require a human.
- Issued credentials are leases with TTLs; revoking cascades to leases,
  credential sessions and agent tokens.
- Derived keys (`valtag_dk_…`) are HMAC-bound to the master key and are only
  meaningful to valtag's gateway.

## Rate limiting

- Per-agent limiter with `RateLimit-*` / `Retry-After` headers
  ([ax.md](ax.md)). Redis errors fail closed (503) unless
  `RATE_LIMIT_FAIL_OPEN=true`. With `REDIS_URL` empty the agent limiter is
  disabled and the server logs a warning.
- Exact limits live in code (`server/internal/ratelimit/`,
  `server/internal/middleware/rate-limit.go`); they are not restated here.

## Audit

- Every event is stored in a SHA-256 hash chain, partitioned by month in
  PostgreSQL. Editing any row breaks `GET /audit/verify`; keep a recent head
  off-site (`?expected_head=`) to also detect tail truncation.
- Each gateway authorize call is recorded in the chain.

## Threat mitigations

| Threat | Mitigation | Limit |
|---|---|---|
| Compromised AI agent | Short-lived leases, policy-gated approval, one-command revoke cascade | A secret with no bound policy auto-approves at the default tier |
| Database-only breach (SQL dump) | Blobs and DEKs are ciphertext; the master key is not in the database | Attacker with the DB **and** the master key reads everything |
| Compromised server host / malicious operator | Hash-chained audit, off-site head anchoring, host hardening | **Not prevented by encryption** — the server decrypts to deliver |
| Leaked lease key | TTL expiry, revoke; derived keys are useless outside valtag's gateway | Valid until expiry or revoke |
| Brute force | Argon2id, login lockout, rate limiting | — |
| Audit tampering | SHA-256 hash chain + `GET /audit/verify` with off-site anchor | Without an anchor, tail truncation is not detectable |
