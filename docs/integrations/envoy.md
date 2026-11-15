# valtag × Envoy — ext_authz HTTP filter

Envoy's `ext_authz` HTTP filter is the mechanism Higress's ext-auth plugin
wraps — so a raw Envoy deployment integrates with the exact same contract
this repo already runs live (verified on the staging Higress deployment).

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
                        uri: 127.0.0.1:10256
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

- `path_prefix` makes Envoy send the original request to
  `POST /gateway/v1/authorize/<original-path>` — the authorize endpoint
  reads headers only and ignores the suffix.
- `allowed_upstream_headers` copies valtag's response headers onto the
  forwarded request — the credential replaces the derived key.
- 401/403 from valtag → Envoy denies without touching the upstream
  (fail-closed by default: `clear_route_cache`, no `failure_mode_allow`).

## Verify

```bash
curl -H "Authorization: Bearer valtag_dk_…" http://127.0.0.1:10000/   # → 200
curl http://127.0.0.1:10000/                                          # → 401
```

The staging Higress deployment runs this exact filter contract through the
Higress ext-auth plugin — same endpoint, same headers, same fail-closed
semantics (verified end-to-end, see [demo/higress](../../demo/higress/README.md)).
