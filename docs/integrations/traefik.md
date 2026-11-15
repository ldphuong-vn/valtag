# valtag × Traefik — forwardAuth middleware

Traefik's `forwardAuth` middleware forwards the original request (method and
headers) to the auth endpoint and gates on the response status.

```yaml
# Traefik dynamic configuration (file provider)
http:
  middlewares:
    valtag:
      forwardAuth:
        address: "http://127.0.0.1:10256/gateway/v1/authorize"
        authResponseHeaders:
          - Authorization   # valtag's real credential replaces the derived key

  routers:
    app:
      rule: "Host(`app.example.com`)"
      middlewares:
        - valtag
      service: app
```

Docker labels flavour:

```yaml
    labels:
      - "traefik.http.middlewares.valtag.forwardauth.address=http://127.0.0.1:10256/gateway/v1/authorize"
      - "traefik.http.middlewares.valtag.forwardauth.authResponseHeaders=Authorization"
      - "traefik.http.routers.app.middlewares=valtag"
```

## Verify

```bash
# with a lease-derived key → 200 (the upstream receives the real credential)
curl -H "Authorization: Bearer valtag_dk_…" https://app.example.com/

# without / with a dead key → 401 (fail-closed)
curl https://app.example.com/          # → 401
```

## Notes

- `forwardAuth` forwards the original request as-is — the authorize endpoint
  is method-agnostic, so GET/POST both work.
- `authResponseHeaders: [Authorization]` copies valtag's credential header
  onto the forwarded request — without it the upstream would see the derived
  key.
- Keep the valtag gateway listener on loopback/private — Traefik is the only
  client that should reach `:10256`.
