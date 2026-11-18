# valtag × Envoy — ext_authz HTTP filter

> **Status: draft — not yet validated or run against valtag.** Known gap:
> Envoy's HTTP `ext_authz` sends the check to `path_prefix` **plus the
> original request path** (for a request to `/`, that is
> `/gateway/v1/authorize/`), while valtag's endpoint currently matches only
> the exact path `/gateway/v1/authorize`
> (`server/internal/gateway/server.go:78`). As written, every request would
> be denied (fail-closed). The endpoint has to accept the suffix before this
> page is usable. Until then use nginx, Caddy or Higress — those are
> live-tested.

Envoy's `ext_authz` HTTP filter calls an external HTTP service before
forwarding a request and can copy headers from the response onto the
forwarded request — the same shape valtag's authorize endpoint exposes.

```yaml
# envoy.yaml — minimal sidecar/gateway with valtag as the auth gate
static_resources:
  listeners:
    - name: listener_0
      address: { socket_address: { address: 0.0.0.0, port_value: 10000 } }
      filter_chains:
        - filters:
            - name: envoy.filters.network.http_connection_manager
              typed_config:
                "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
                stat_prefix: ingress
                route_config:
                  virtual_hosts:
                    - name: protected
                      domains: ["*"]
                      routes:
                        - match: { prefix: "/" }
                          route: { cluster: protected_app }
                http_filters:
                  - name: envoy.filters.http.ext_authz
                    typed_config:
                      "@type": type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz
                      http_service:
                        server_uri:
                          uri: 127.0.0.1:10256
                          cluster: valtag_authorize
                          timeout: 2s
                        path_prefix: /gateway/v1/authorize
                        authorization_request:
                          allowed_headers:
                            patterns: [{ exact: authorization }]
                        authorization_response:
                          allowed_upstream_headers:
                            patterns: [{ exact: authorization }, { prefix: x-valtag- }]
                  - name: envoy.filters.http.router
                    typed_config:
                      "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
  clusters:
    - name: protected_app
      connect_timeout: 5s
      type: STATIC
      load_assignment:
        cluster_name: protected_app
        endpoints: [{ lb_endpoints: [{ endpoint: { address: { socket_address: { address: 127.0.0.1, port_value: 3000 } } } }] }]
    # The auth cluster — valtag's gateway listener.
    - name: valtag_authorize
      connect_timeout: 5s
      type: STATIC
      load_assignment:
        cluster_name: valtag_authorize
        endpoints: [{ lb_endpoints: [{ endpoint: { address: { socket_address: { address: 127.0.0.1, port_value: 10256 } } } }] }]
```

- `server_uri` points the filter at the `valtag_authorize` cluster below.
- `path_prefix` — Envoy appends the original request path to it (see the
  status note above).
- `allowed_upstream_headers` copies valtag's response headers onto the
  forwarded request — the credential replaces the derived key.
- A 401/403 from valtag → Envoy denies without touching the upstream.
  `failure_mode_allow` is left at its default (`false`), so an unreachable
  valtag also denies (fail-closed).

## Verify (once the path gap is closed)

```bash
curl -H "Authorization: Bearer valtag_dk_…" http://127.0.0.1:10000/   # → 200
curl http://127.0.0.1:10000/                                          # → 401
```

The Higress deployment in [demo/higress](../../demo/higress/README.md) uses
the same HTTP contract, but through Higress's ext-auth plugin in
`forward_auth` mode, which calls the configured path as-is. That is a
different code path from Envoy's `ext_authz` filter, so it does not verify
this page.
