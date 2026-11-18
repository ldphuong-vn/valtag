# valtag × Traefik — forwardAuth middleware

> ✅ **Verified live** (Traefik v3.3, staging 2026-11-18/19): auth-request
> shape, full matrix, header-strip test, fail-closed when valtag is down.

Traefik's `forwardAuth` middleware delegates authentication to an external
service and can copy headers from the auth response onto the forwarded
request.

```yaml
# Traefik dynamic configuration (file provider)
http:
  middlewares:
    # strip client-sent valtag headers BEFORE the auth middleware
    strip-valtag:
      headers:
        customRequestHeaders:
          x-valtag-agent-id: ""
          x-valtag-lease-id: ""
          x-valtag-lease-expires: ""
    valtag:
      forwardAuth:
        address: "http://127.0.0.1:10256/gateway/v1/authorize"
        authResponseHeaders:
          - Authorization   # valtag's real credential replaces the derived key

  routers:
    app:
      rule: "Host(`app.example.com`)"
      middlewares:
        - strip-valtag      # before forwardAuth in the chain
        - valtag
      service: app
```

Docker labels flavour:

```yaml
    labels:
      - "traefik.http.middlewares.valtag.forwardauth.address=http://127.0.0.1:10256/gateway/v1/authorize"
      - "traefik.http.middlewares.valtag.forwardauth.authResponseHeaders=Authorization"
      - "traefik.http.middlewares.strip-valtag.headers.customrequestheaders.x-valtag-agent-id="
      - "traefik.http.routers.app.middlewares=strip-valtag,valtag"
```

## What was verified live

Measured with an intermediate logger and an echo upstream
(`traefik/whoami`):

- **Auth-request shape**: Traefik sends the auth request to the configured
  address **as-is** (`/gateway/v1/authorize` — no original path appended, so
  no server-side change is needed) with the client's `Authorization` header
  forwarded. The **original method, path and body are preserved** on the
  forwarded request (a POST with body stays a POST with body).
- **Matrix**: valid lease → 200 with the upstream receiving the swapped
  credential (`Authorization: Bearer <master_key>`); no key and dead lease →
  401 (valtag's problem+json relayed to the client); **valtag down → 500**
  from Traefik — fail-closed, never open.
- **Strip test**: a client-sent fake `x-valtag-agent-id` and the derived key
  show **0 occurrences** at the upstream when the `strip-valtag` middleware
  precedes `forwardAuth` in the chain.

## Verify

```bash
# with a lease-derived key → 200 (the upstream receives the real credential)
curl -H "Authorization: Bearer valtag_dk_…" https://app.example.com/

# without / with a dead key → 401 (fail-closed)
curl https://app.example.com/          # → 401
```

## Notes

- `forwardAuth` forwards **all client headers** to the auth service —
  including `x-valtag-*` sent by a client. valtag's authorize endpoint
  ignores them, but strip them at the edge anyway (middleware above) so they
  never reach the upstream.
- `authResponseHeaders: [Authorization]` copies valtag's credential header
  onto the forwarded request — without it the upstream would see the derived
  key.
- Keep the valtag gateway listener on loopback/private — Traefik is the only
  client that should reach `:10256`.
