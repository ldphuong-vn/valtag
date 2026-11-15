# valtag × gateway integrations — forward-auth compatibility

valtag's gateway listener speaks a simple HTTP contract any reverse proxy with
a forward-auth mechanism can integrate **with config only — no plugins**:

```text
Authorization: Bearer valtag_dk_…  ──▶  POST|GET /gateway/v1/authorize
                                        ◀── 200 + credential as headers
                                        ◀── 401/403 (fail-closed)
```

- The authorize endpoint is **method-agnostic** (POST, GET — subrequest flavors
  included; `5aca731`).
- A `200` response carries the **real upstream credential** in the
  `Authorization` response header plus trace headers (`x-valtag-*`) — copy it
  onto the forwarded request and the backend sees the credential, never the
  derived key.
- Non-2xx = deny. If valtag is unreachable the edge must **fail-closed**.
- Only headers are read — no body forwarding required.

## Compatibility matrix

| Gateway | Mechanism | Example | Verified |
| --- | --- | --- | --- |
| Higress | ext-auth plugin (forward_auth) | [demo/higress](../../demo/higress/README.md) | ✅ live staging |
| Envoy (raw) | ext_authz HTTP filter | [envoy.md](envoy.md) | ✅ same filter contract as Higress |
| nginx | auth_request module | [nginx.md](nginx.md) | ✅ container test |
| Caddy | forward_auth handler | [caddy.md](caddy.md) | ✅ container test |
| Traefik | forwardAuth middleware | [traefik.md](traefik.md) | ✅ container test |
| Apache APISIX | forward-auth plugin | [apisix.md](apisix.md) | config mapped from docs |

## Runtime prerequisites (every gateway)

1. valtag deployed with `GATEWAY_ENABLED=true`, `GATEWAY_PORT=10256` — the
   listener must be reachable from the gateway over loopback/private network,
   never the public internet.
2. A lease for the calling agent: bind a policy template to the secret
   (require_approval = human-in-the-loop) or rely on auto-approve; mint via
   MCP (`request_secret_access`), CLI (`valtag get`) or
   `POST /api/v1/providers/{id}/leases`.
3. TLS in one place (edge or valtag's upstream), per
   [deployment guide §TLS](../deployment-guide.md#2-production-on-a-single-vps).

## Security notes

- The authorize endpoint accepts **any HTTP method** and reads only headers —
  keep the listener on loopback/private networks; it is a machine surface.
- Response headers carry the real credential — only copy them onto requests
  the policy has approved (all supported mechanisms above copy into the
  forwarded request, not to the client).
- Behind Cloudflare, client-IP attribution in the audit chain uses
  `CF-Connecting-IP` (trust `TRUST_PROXY_XFF` on the valtag server).
