#!/usr/bin/env bash
# Smoke test demo valtag × Higress — chạy TRÊN SERVER (103.142.137.130).
#
# Dùng:
#   Phase H (key-auth tĩnh):  AUTH_MODE=on AUTH_KEY=REPLACE_ME_STATIC_KEY_DEMO AUTH_HEADER=x-api-key ./smoke.sh
#   Phase V (valtag ext-auth): AUTH_MODE=on AUTH_KEY=valtag_dk_xxx AUTH_HEADER=Authorization ./smoke.sh
#   Kiểm fail-closed:          ... ./smoke.sh --failclosed
#   AUTH_MODE=on  → không key phải bị chặn (401/403)
#   AUTH_MODE=none→ không key phải được qua (route trần, trước khi bật plugin)
#
# Thoát khác 0 nếu có check FAIL.

set -u
GW="${GATEWAY:-http://127.0.0.1:${HIGRESS_HTTP_PORT:-8880}}"
AUTH_KEY="${AUTH_KEY:-}"
AUTH_HEADER="${AUTH_HEADER:-x-api-key}"
AUTH_MODE="${AUTH_MODE:-none}"
FAIL=0

say()   { printf '\n=== %s ===\n' "$1"; }
ck()    { if [ "$1" = "$2" ]; then echo "  PASS ($1)"; else echo "  FAIL (kỳ vọng $2, nhận $1)"; FAIL=1; fi; }
ck_in() { if [[ "$1" =~ ^($2)$ ]]; then echo "  PASS ($1)"; else echo "  FAIL (kỳ vọng $2, nhận $1)"; FAIL=1; fi; }
lat()   { for i in 1 2 3 4 5; do curl -s -o /dev/null -w '%{time_total}\n' "$@"; done | awk '{printf "  %.0f ms\n", $1*1000}'; }
# 401: auth service từ chối key (pass-through status); 403: ext-auth chặn khi không chạm được service.
REJECT='401|403'

# Authorization header phải có scheme: valtag bearerTokenFrom đòi "Bearer <key>".
# Phase H (x-api-key) thì raw value. Tự thêm "Bearer " nếu thiếu.
AUTH_PREFIX=""
if [ "$AUTH_HEADER" = "Authorization" ] && [ -n "$AUTH_KEY" ] && [ "${AUTH_KEY#* }" = "$AUTH_KEY" ]; then
  AUTH_PREFIX="Bearer "
fi
auth_header_arg() { printf '%s: %s%s' "$AUTH_HEADER" "$AUTH_PREFIX" "$1"; }

say "1) Route /whoami phản hồi khi có key (Higress sống, route + plugin hoạt động)"
code=$(curl -s -o /dev/null -w '%{http_code}' -H "$(auth_header_arg "${AUTH_KEY:-dummy}")" "$GW/whoami")
if [ "$AUTH_MODE" = "on" ] && [ -n "$AUTH_KEY" ]; then ck "$code" 200; else ck_in "$code" '200|401|403'; fi

say "2) Không có key — AUTH_MODE=$AUTH_MODE"
code=$(curl -s -o /dev/null -w '%{http_code}' "$GW/whoami")
if [ "$AUTH_MODE" = "on" ]; then ck_in "$code" "$REJECT"; else ck "$code" 200; fi

if [ "$AUTH_MODE" = "on" ] && [ -n "$AUTH_KEY" ]; then
  say "3) Key SAI — phải bị chặn, không chạm upstream"
  code=$(curl -s -o /dev/null -w '%{http_code}' -H "$(auth_header_arg "key-sai-tinh-y")" "$GW/whoami")
  ck_in "$code" "$REJECT"

  say "4) Key HỢP LỆ — 200 + credential đến đích: Phase V có x-valtag-lease-id (key bị THAY bằng credential thật), Phase H echo lại key tĩnh"
  body=$(curl -s -H "$(auth_header_arg "$AUTH_KEY")" "$GW/whoami")
  echo "$body" | grep -iE "x-valtag-lease-id|x-api-key|authorization" | head -3
  if echo "$body" | grep -qi "x-valtag-lease-id"; then ck 200 200
  elif echo "$body" | grep -q -F "$AUTH_KEY"; then ck 200 200
  else echo "  FAIL (body không có bằng chứng credential)"; FAIL=1; fi

  say "5) Latency 5 request có auth (Phase V sẽ +1 RTT qua valtag — so với Phase H)"
  lat -H "$(auth_header_arg "$AUTH_KEY")" "$GW/whoami"
fi

if [ "${1:-}" = "--failclosed" ]; then
  say "6) FAIL-CLOSED: dừng valtag server → request phải 403 (không được 200/502)"
  VALTAG_CONTAINER=$(docker ps --format '{{.Names}}' | grep -E 'valtag|vaultsaas' | grep server | head -1)
  echo "  dừng: $VALTAG_CONTAINER"
  docker stop "$VALTAG_CONTAINER" >/dev/null
  sleep 2
  code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: ${AUTH_KEY:-valtag_dk_dummy}" "$GW/whoami")
  docker start "$VALTAG_CONTAINER" >/dev/null
  ck "$code" 403
fi

say "7) Isolation: path lạ cùng host không bị plugin đụng tới (kỳ vọng 404, không 401/403)"
code=$(curl -s -o /dev/null -w '%{http_code}' "$GW/nonexistent-path")
ck_in "$code" '404'

[ "$FAIL" = 0 ] && echo -e "\n>>> SMOKE OK" || { echo -e "\n>>> SMOKE FAIL"; exit 1; }
