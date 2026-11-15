# valtag × Caddy — forward_auth integration

Caddy's `forward_auth` handler clones the original request (headers included)
into an auth subrequest and gates on the response status. valtag's authorize
endpoint is method-agnostic, so the default subrequest works as-is.

```text
# /etc/caddy/Caddyfile
app.example.com {
    forward_auth 127.0.0.1:10256 {
        uri /gateway/v1/authorize
        copy_headers Authorization   # valtag's real credential → upstream request
    }

    reverse_proxy 127.0.0.1:3000     # the protected app
}
```

## Verify

```bash
# with a lease-derived key → 200 (the upstream receives the real credential)
curl -H "Authorization: Bearer valtag_dk_…" https://app.example.com/

# without / with a dead key → 401 (fail-closed)
curl https://app.example.com/          # → 401
```

## Notes

- `forward_auth` subrequests carry the original client headers, including
  `Authorization` — no extra header mapping needed.
- `copy_headers Authorization` copies valtag's response header onto the
  forwarded request; without it the upstream would see the derived key
  instead of the real credential.
- Keep the valtag gateway listener on loopback/private — Caddy is the only
  client that should reach `:10256`.
