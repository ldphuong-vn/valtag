#!/bin/bash
# valtag lease sidecar for LiteLLM's `api_key: oidc/file//…` upstream keys.
#
# Mint one lease from valtag and atomically replace the lease file LiteLLM
# re-reads on every call. Run it as a loop next to LiteLLM:
#
#   export VALTAG_API_URL=https://valt.turbo.ai.vn        # or internal URL
#   export VALTAG_AGENT_TOKEN=<agent token>               # Dashboard → Agents → Issue Token
#   export VALTAG_AGENT_ID=<agent id>
#   export VALTAG_PROVIDER_ID=<derived-key provider id>
#   export LEASE_FILE=/run/secrets/valtag-lease
#   ./oidc_file_sidecar.sh loop            # forever, re-minting before expiry
#   ./oidc_file_sidecar.sh                 # one mint (for testing)
#
# Config side (litellm config.yaml):
#   - model_name: my-model
#     litellm_params:
#       model: openai/…
#       api_key: "oidc/file//run/secrets/valtag-lease"
#
# NOTE: this is ONE shared lease for the whole proxy — every caller of
# LiteLLM uses the same upstream credential. Per-caller isolation is the
# inbound gate's job (custom_auth), not this sidecar's.
set -eu

MODE="${1:-once}"
LEASE_FILE="${LEASE_FILE:-/run/secrets/valtag-lease}"
LEASE_TTL="${LEASE_TTL:-300}"
LEASE_SLEEP="${LEASE_SLEEP:-$((LEASE_TTL - 60))}"   # re-mint a minute before expiry
VALTAG_API_URL="${VALTAG_API_URL:-http://127.0.0.1:8080}"

mint() {
  RESP=$(curl -s --max-time 8 -X POST "$VALTAG_API_URL/api/v1/providers/$VALTAG_PROVIDER_ID/leases" \
    -H "Authorization: Bearer $VALTAG_AGENT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"agent_id\":\"$VALTAG_AGENT_ID\",\"ttl_seconds\":$LEASE_TTL}")
  KEY=$(printf '%s' "$RESP" | sed -n 's/.*"api_key":"\([^"]*\)".*/\1/p')
  if [ -z "$KEY" ]; then
    echo "sidecar: mint failed: $RESP" >&2
    return 1
  fi
  DIR=$(dirname "$LEASE_FILE")
  printf '%s' "$KEY" > "$LEASE_FILE.tmp"
  mv "$LEASE_FILE.tmp" "$LEASE_FILE"   # atomic — readers never see a partial file
  echo "sidecar: minted ${KEY:0:14}… ttl=${LEASE_TTL}s"
}

while :; do
  mint || true
  [ "$MODE" = "once" ] && break
  sleep "$LEASE_SLEEP"
done
