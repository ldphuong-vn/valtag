# valtag × nginx — auth_request integration

nginx's `auth_request` module makes a subrequest to an auth endpoint and
gates the original request on its status. valtag's authorize endpoint is
method-agnostic, so the GET subrequest works as-is.

```nginx
# /etc/nginx/conf.d/valtag.conf
server {
    listen 80;
    server_name app.example.com;

    # The protected surface — anything under this location requires a
    # valtag-approved credential.
    location / {
        auth_request /_valtag_auth;

        # Hand the real credential to the upstream (from valtag's response).
        auth_request_set $valtag_credential $upstream_http_authorization;
        proxy_set_header Authorization $valtag_credential;

        proxy_pass http://127.0.0.1:3000;   # the protected app
    }

    # Internal auth subrequest — hidden from clients.
    location = /_valtag_auth {
        internal;
        proxy_pass http://127.0.0.1:10256/gateway/v1/authorize;
        proxy_pass_request_body off;        # contract reads headers only
        proxy_set_header Content-Length "";
        proxy_set_header Authorization $http_authorization;
        proxy_set_header X-Forwarded-For $remote_addr;
    }
}
```

## Verify

```bash
# with a lease-derived key → 200 (the upstream receives the real credential)
curl -H "Authorization: Bearer valtag_dk_…" http://app.example.com/

# without / with a dead key → 401 (nginx surfaces the auth subrequest status)
curl http://app.example.com/          # → 401
```

## Notes

- `auth_request` subrequests send **GET** with the original client headers —
  the authorize endpoint is method-agnostic (`5aca731`) and only reads the
  `Authorization` header, so no body handling is needed.
- `auth_request_set` + `proxy_set_header Authorization $upstream_http_…` is
  how the credential replaces the derived key before the upstream hop.
- Keep the valtag gateway listener on loopback/private — nginx is the only
  client that should reach `:10256`.
