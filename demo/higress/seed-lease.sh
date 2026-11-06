#!/usr/bin/env bash
# Seed demo data in valtag and mint a lease. Idempotent: reuses the org /
# workspace / project / provider when they already exist.
#
# Output (stderr): human-readable progress.
# Output (stdout): eval-able assignments —
#   VALTAG_PROVIDER_ID=... VALTAG_KEY=valtag_dk_... VALTAG_LEASE_ID=...
# so `eval "$(./seed-lease.sh)"` just works.
#
# Env: VALTAG_API (default http://127.0.0.1:8080/api/v1), ADMIN_EMAIL,
# ADMIN_PASS (seed default admin@valt.dev / ValtAdmin2026! — CHANGE IT before
# any real use), MASTER_KEY (the fake "real upstream credential" this demo
# hands out through the gateway), HIGRESS_CONTAINER.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
API="${VALTAG_API:-http://127.0.0.1:8080/api/v1}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@valt.dev}"
ADMIN_PASS="${ADMIN_PASS:-ValtAdmin2026!}"
MASTER_KEY="${MASTER_KEY:-sk-upstream-real-credential-demo}"
ORG_NAME="valtag-demo-org";   ORG_SLUG="valtag-demo"
WS_NAME="valtag-demo-ws";     WS_SLUG="valtag-demo"
PROJ_NAME="valtag-demo-project"; PROJ_SLUG="valtag-demo"
PROVIDER_NAME="higress-demo-upstream"

resolve_container() {
  if [ -n "${HIGRESS_CONTAINER:-}" ]; then echo "$HIGRESS_CONTAINER"; return; fi
  (cd "$DIR" && docker compose ps -q higress | head -1)
}
CT="$(resolve_container)"
[ -n "$CT" ] || { echo "ERROR: higress container not found" >&2; exit 1; }

# JSON field extraction with the yq binary inside the Higress container —
# keeps this script dependency-free on the host (bash + curl + docker only).
yget() { docker exec -i "$CT" yq -r "$1" | sed 's/^null$/ /' | tr -d '\n'; }

info() { echo "$*" >&2; }

TOKEN=$(curl -s -X POST "$API/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASS\"}" | yget '.access_token // ""')
[ -n "$TOKEN" ] && [ "$TOKEN" != " " ] || { echo "ERROR: login failed — check ADMIN_EMAIL/ADMIN_PASS and VALTAG_API" >&2; exit 1; }
AUTH="Authorization: Bearer $TOKEN"

find_id() { # find_id <list-url> <name>
  curl -s "$1" -H "$AUTH" | yget ".[] | select(.name == \"$2\") | .id // \"\"" 2>/dev/null || true
}
create() { # create <url> <json>
  curl -s -X POST "$1" -H "$AUTH" -H 'Content-Type: application/json' -d "$2" | yget '.id // ""'
}

ORG_ID=$(find_id "$API/orgs" "$ORG_NAME")
[ -n "$ORG_ID" ] || ORG_ID=$(create "$API/orgs" "{\"name\":\"$ORG_NAME\",\"slug\":\"$ORG_SLUG\"}")
[ -n "$ORG_ID" ] || { echo "ERROR: org create failed" >&2; exit 1; }
info "org: $ORG_ID"

WS_ID=$(find_id "$API/orgs/$ORG_ID/workspaces" "$WS_NAME")
[ -n "$WS_ID" ] || WS_ID=$(create "$API/orgs/$ORG_ID/workspaces" "{\"name\":\"$WS_NAME\",\"slug\":\"$WS_SLUG\"}")
[ -n "$WS_ID" ] || { echo "ERROR: workspace create failed" >&2; exit 1; }

PROJ_ID=$(find_id "$API/workspaces/$WS_ID/projects" "$PROJ_NAME")
[ -n "$PROJ_ID" ] || PROJ_ID=$(create "$API/workspaces/$WS_ID/projects" "{\"name\":\"$PROJ_NAME\",\"slug\":\"$PROJ_SLUG\"}")
[ -n "$PROJ_ID" ] || { echo "ERROR: project create failed" >&2; exit 1; }
info "project: $PROJ_ID"

PROVIDER_ID=$(find_id "$API/projects/$PROJ_ID/providers" "$PROVIDER_NAME")
[ -n "$PROVIDER_ID" ] || PROVIDER_ID=$(create "$API/projects/$PROJ_ID/providers" \
  "{\"name\":\"$PROVIDER_NAME\",\"provider_type\":\"derived_api_key\",\"config\":{\"master_key\":\"$MASTER_KEY\",\"injection_type\":\"bearer\"}}")
[ -n "$PROVIDER_ID" ] || { echo "ERROR: provider create failed" >&2; exit 1; }
info "provider: $PROVIDER_ID"

LEASE=$(curl -s -X POST "$API/providers/$PROVIDER_ID/leases" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"ttl_seconds":3600}')
KEY=$(printf '%s' "$LEASE" | yget '.credentials.api_key // ""')
LEASE_ID=$(printf '%s' "$LEASE" | yget '.id // ""')
[ -n "$KEY" ] || { echo "ERROR: lease mint failed: $LEASE" >&2; exit 1; }
info "lease: $LEASE_ID (expires in 1h)"

printf 'VALTAG_PROVIDER_ID=%s\nVALTAG_KEY=%s\nVALTAG_LEASE_ID=%s\n' "$PROVIDER_ID" "$KEY" "$LEASE_ID"
