#!/usr/bin/env bash
# The 60-second demo: ask → approve → lease → use → revoke → dead key.
#
#   1. Mint a lease (in a real deployment a human approves this in the
#      dashboard / Telegram / Slack — see the valtag README).
#   2. Call the gateway with the derived key  → 200, upstream gets the REAL
#      credential, the derived key is replaced at the gateway.
#   3. Revoke the lease.
#   4. Same key again → 401. Revocation is instant, nothing to redeploy.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
GW="${GATEWAY:-http://127.0.0.1:${HIGRESS_HTTP_PORT:-8880}}"
API="${VALTAG_API:-http://127.0.0.1:8080/api/v1}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@valt.dev}"
ADMIN_PASS="${ADMIN_PASS:-ValtAdmin2026!}"

echo "── 1) mint a lease (human approval step omitted for the demo) ──"
eval "$("$DIR/seed-lease.sh")"
echo "   derived key: ${VALTAG_KEY:0:16}… (lease $VALTAG_LEASE_ID, TTL 1h)"

echo
echo "── 2) call the gateway with the derived key ──"
code=$(curl -s -o /tmp/demo-body.$$ -w '%{http_code}' --max-time 15 \
  -H "Authorization: Bearer $VALTAG_KEY" "$GW/whoami")
echo "   gateway → $code"
grep -iE '^(authorization|x-valtag)' /tmp/demo-body.$$ | sed 's/^/   upstream received: /'
rm -f /tmp/demo-body.$$
[ "$code" = "200" ] || { echo "   FAILED (expected 200)"; exit 1; }

echo
echo "── 3) revoke the lease ──"
TOKEN=$(curl -s -X POST "$API/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASS\"}" \
  | docker exec -i "$(cd "$DIR" && docker compose ps -q higress | head -1)" yq -r '.access_token // ""')
code=$(curl -s -o /dev/null -w '%{http_code}' -X DELETE --max-time 15 \
  -H "Authorization: Bearer $TOKEN" "$API/leases/$VALTAG_LEASE_ID")
echo "   revoke → $code"

echo
echo "── 4) same key again ──"
code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 15 \
  -H "Authorization: Bearer $VALTAG_KEY" "$GW/whoami")
echo "   gateway → $code"
[ "$code" = "401" ] || { echo "   FAILED (expected 401 after revoke)"; exit 1; }

echo
echo ">>> DEMO OK: credential was live per-lease, and died the moment the lease was revoked."
echo "    No gateway config changed. No static credential anywhere in the gateway."
