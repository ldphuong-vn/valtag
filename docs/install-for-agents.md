# Install with an agent

To have a coding agent (Claude Code, Codex, Cursor or another) install or
deploy valtag, paste this prompt into it:

```text
Install valtag for me by following
https://raw.githubusercontent.com/ldphuong-vn/valtag/main/docs/install-for-agents.md
```

The agent inspects the host, asks before anything it cannot undo, and reports
the console address, service URLs and credential locations when it is done.
The rest of this page is written for the agent.

## For the agent

Use the same [deployment guide](deployment-guide.md) as a human operator — it
owns the commands, Compose files, key generation and supported paths. This
page adds the agent's preflight, authorization, verification and handoff
rules. Also read [CONTEXT.md](../CONTEXT.md) for domain vocabulary before
starting.

### Inspect before acting

Determine whether valtag already exists on the target host: running containers
(`docker ps`), compose files and their project names, Postgres instances,
occupied host ports, data directories (`/data` mounts, `./data` binds, `./keys`)
and existing reverse proxies. Inspect container names, image references, volume
mounts and network metadata — **never print container environments, `.env`
contents, `VAULT_MASTER_KEY`, `SERVER_KEK`, JWT private keys or decrypted
credential values in tool output.** Ask the user only for facts that cannot be
established safely from the host.

When an existing Compose file is present, read its `services` definitions
before assuming `.env` variables take effect: hand-written files may hardcode
host port bindings, bind mounts or CORS origins without variable
interpolation — inspect the file first rather than setting variables the file
ignores.

### Preserve the existing installation

- Get user approval before overwriting any existing file (compose, `.env`,
  `keys/`) or changing host port bindings, bind mounts, named volumes,
  networks or reverse-proxy rules. Switching an existing bind mount to a named
  volume (or vice versa) can orphan the entire vault state — verify the
  running container's actual mounts with `docker inspect` before any change.
- **Never replace an existing `VAULT_MASTER_KEY` or `SERVER_KEK`.** If the
  master key is lost, encrypted secrets are unrecoverable — stop and explain
  recovery limits ([the rotation runbook](key-rotation.md)) before touching
  the deployment.
- Let the user supply missing secrets privately; placeholders are not working
  credentials.
- Do not stop, restart or reconfigure an unrelated service (an AI gateway,
  another panel, a different project's containers) without explicit user
  authorization. If the deployment shares a host with other stacks, check
  port collisions and docker network ownership before recreating containers —
  recreating a container silently drops manually-attached docker networks.
- Get explicit approval for privilege elevation (`sudo`), package installs,
  firewall changes and DNS changes.

### Deploy paths

| Goal | Path |
| --- | --- |
| Local development | [deployment-guide.md §1](deployment-guide.md#1-local-development) — `make dev`, migrations, seed |
| Production single VPS | [deployment-guide.md §2](deployment-guide.md#2-production-on-a-single-vps) — compose, keys, TLS, hardening |
| Behind an AI gateway (Higress) | [demo/higress/README.md](../demo/higress/README.md) — ext-auth wiring, fail-closed |

### Verification (every path)

1. `GET /health` → 200.
2. `docker compose ps` — all services Up; postgres healthy.
3. Migrations applied: `valtag-migrate up` is idempotent and also ensures
   `audit_logs` partitions.
4. Login works and **Sign out** returns the user to `/login` with cookies
   cleared (regression: eaed991).
5. If the gateway listener is enabled: an unauthorized
   `POST /gateway/v1/authorize` returns `401` with an RFC 9457 problem body —
   fail-closed is the contract (see [ax.md](ax.md)).

### Handoff

Report to the operator: service URLs (dashboard, API, gateway listener if
enabled), which env files hold which secrets, where the master key is stored
(off the server), how backups and key rotation work
([key-rotation.md](key-rotation.md)), and the restart policy of every
container. Do not report decrypted values, key material or database contents.
