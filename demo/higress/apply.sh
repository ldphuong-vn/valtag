#!/usr/bin/env bash
# Apply the demo configuration to the Higress all-in-one container via its
# internal apiserver — no console interaction needed.
#
#   ./apply.sh            # "after" state: ext-auth → valtag on the /whoami route
#   ./apply.sh --before   # "before" state: static key-auth instead of valtag
#
# Idempotent: re-runs update the existing resources (spec is replaced,
# resourceVersion is carried over). Requires only docker + the running stack
# (JSON parsing happens with the yq binary shipped inside the Higress image).
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
MODE="${1:---after}"
API="https://localhost:18443"
NS="higress-system"

resolve_container() {
  if [ -n "${HIGRESS_CONTAINER:-}" ]; then echo "$HIGRESS_CONTAINER"; return; fi
  (cd "$DIR" && docker compose ps -q higress | head -1)
}
CT="$(resolve_container)"
[ -n "$CT" ] || { echo "ERROR: higress container not found — is the stack up? (cd $(basename "$DIR") && docker compose up -d)"; exit 1; }

push_file() { docker exec -i "$CT" sh -c "cat > '$1'"; } # push_file <container-path> < local

echo "higress container: $CT"
echo "waiting for the internal apiserver..."
READY=0
for _ in $(seq 1 60); do
  code=$(docker exec "$CT" curl -sk -o /dev/null -w '%{http_code}' "$API/apis" 2>/dev/null || true)
  if [ "$code" = "200" ]; then READY=1; break; fi
  sleep 2
done
[ "$READY" = 1 ] || { echo "ERROR: apiserver never became ready"; exit 1; }

# apply_resource <collection-path> <item-path> <template-file>
# POST the template to the collection; when the resource already exists,
# replace its spec with the template's (metadata/resourceVersion come from the
# live object) and PUT to the item path.
apply_resource() {
  local coll="$1" item="$2" file="$3" res
  push_file /tmp/cr.json < "$file"
  res=$(docker exec "$CT" curl -sk -X POST -H 'Content-Type: application/json' \
    --data-binary @/tmp/cr.json "$API$coll")
  if ! printf '%s' "$res" | grep -q '"kind": *"Status"'; then
    echo "  created  $(basename "$file")"
    return 0
  fi
  # exists (or POST rejected) → inspect the live object
  docker exec "$CT" curl -sk "$API$item" | push_file /tmp/live.json
  if docker exec "$CT" grep -q '"kind": *"Status"' /tmp/live.json; then
    echo "  FAILED   $(basename "$file"): POST said: $res"
    return 1
  fi
  docker exec "$CT" sh -c "yq eval-all 'select(fileIndex == 0).spec = select(fileIndex == 1).spec | select(fileIndex == 0)' -o=json /tmp/live.json /tmp/cr.json > /tmp/merged.json"
  res=$(docker exec "$CT" curl -sk -X PUT -H 'Content-Type: application/json' \
    --data-binary @/tmp/merged.json "$API$item")
  if printf '%s' "$res" | grep -q '"kind": *"Status"'; then
    echo "  FAILED   $(basename "$file"): $res"
    return 1
  fi
  echo "  updated  $(basename "$file")"
}

V1="networking.higress.io/v1"
K8S="networking.k8s.io/v1"
EXT="extensions.higress.io/v1alpha1"
NS_PATH="namespaces/$NS"

echo "applying mode: $MODE"
apply_resource "/apis/$V1/$NS_PATH/mcpbridges" "/apis/$V1/$NS_PATH/mcpbridges/default" "$DIR/cr/mcpbridge.json"
apply_resource "/apis/$K8S/$NS_PATH/ingresses" "/apis/$K8S/$NS_PATH/ingresses/whoami" "$DIR/cr/ingress-whoami.json"

if [ "$MODE" = "--before" ]; then
  apply_resource "/apis/$EXT/$NS_PATH/wasmplugins" "/apis/$EXT/$NS_PATH/wasmplugins/key-auth" "$DIR/cr/wasmplugin-key-auth-before.json"
  apply_resource "/apis/$EXT/$NS_PATH/wasmplugins" "/apis/$EXT/$NS_PATH/wasmplugins/ext-auth" "$DIR/cr/wasmplugin-ext-auth-off.json"
else
  apply_resource "/apis/$EXT/$NS_PATH/wasmplugins" "/apis/$EXT/$NS_PATH/wasmplugins/key-auth" "$DIR/cr/wasmplugin-key-auth.json"
  apply_resource "/apis/$EXT/$NS_PATH/wasmplugins" "/apis/$EXT/$NS_PATH/wasmplugins/ext-auth" "$DIR/cr/wasmplugin-ext-auth.json"
fi

echo "waiting for envoy to pick up the config..."
sleep 6
echo "done. Next: ./seed-lease.sh, then ./demo.sh (or ./smoke.sh)."
