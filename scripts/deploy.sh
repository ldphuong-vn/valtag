#!/bin/bash
# Deploy Valt to VPS
# Usage: ./scripts/deploy.sh

set -e
VPS="root@103.142.137.232"
REMOTE_DIR="/opt/valtag"

echo "=== Deploying Valt ==="

# Sync server
echo "[1/4] Syncing server..."
tar czf - server | ssh $VPS "cd $REMOTE_DIR && rm -rf server && tar xzf -"

# Sync dashboard (exclude node_modules)
echo "[2/4] Syncing dashboard..."
tar czf - --exclude='node_modules' --exclude='.next' --exclude='tsconfig.tsbuildinfo' dashboard | ssh $VPS "cd $REMOTE_DIR && rm -rf dashboard && tar xzf -"

# Sync compose + caddy
echo "[3/4] Syncing config..."
scp docker-compose.prod.yml Caddyfile.domain Caddyfile.ip $VPS:$REMOTE_DIR/

# Rebuild and restart
echo "[4/4] Rebuilding..."
ssh $VPS "cd $REMOTE_DIR && docker compose -f docker-compose.prod.yml down && docker compose -f docker-compose.prod.yml up -d --build"

echo "=== Deploy complete ==="
echo "Dashboard: https://valtag.turbo.ai.vn"
echo "Health: https://valtag.turbo.ai.vn/health"
