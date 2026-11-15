# valtag × Apache APISIX — forward-auth plugin

APISIX's `forward-auth` plugin delegates auth to an external endpoint and
gates on the response status. valtag's authorize endpoint is method-agnostic
and reads only headers, so the default subrequest works as-is.

```yaml
# APISIX route with the valtag gate
routes:
  - uri: /*
    plugins:
      forward-auth:
        uri: "http://127.0.0.1:10256/gateway/v1/authorize"
        request_headers:
          - Authorization
        upstream_headers:
          - Authorization      # valtag's real credential replaces the derived key
    upstream:
      nodes:
        "127.0.0.1:3000": 1
```

- `request_headers: [Authorization]` forwards the agent's derived key
  (`Authorization: Bearer valtag_dk_…`) to valtag.
- `upstream_headers: [Authorization]` copies valtag's response credential
  header onto the forwarded request — the upstream sees the real credential.
- 401/403 from valtag deny the request (fail-closed).

## Verify

```bash
# with a lease-derived key → 200 (the upstream receives the real credential)
curl -H "Authorization: Bearer valtag_dk_…" http://app.example.com/

# without / with a dead key → 401
curl http://app.example.com/          # → 401
```

## Notes

- This page is contract-mapped from the APISIX forward-auth plugin docs and
  the live valtag contract — verify against your APISIX version before
  production (the plugin version's response-header naming differs slightly
  across releases).
- Keep the valtag gateway listener on loopback/private — APISIX is the only
  client that should reach `:10256`.
