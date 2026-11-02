#!/bin/bash
set -e

echo "=== Valt VPS Setup Script ==="
echo "Target: $(hostname) / $(curl -s ifconfig.me 2>/dev/null || echo 'unknown')"
echo ""

# 1. Install Docker
echo "[1/7] Installing Docker..."
if ! command -v docker &>/dev/null; then
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker && systemctl start docker
    echo "Docker installed."
else
    echo "Docker already installed: $(docker --version)"
fi

# 2. Install Docker Compose plugin + Git
echo "[2/7] Installing dependencies..."
apt-get update -qq
apt-get install -y -qq docker-compose-plugin git openssl > /dev/null 2>&1
echo "Dependencies installed."

# 3. Create project directory
echo "[3/7] Setting up project directory..."
mkdir -p /opt/valtag
cd /opt/valtag

# 4. Generate JWT RS256 keys
echo "[4/7] Generating JWT keys..."
mkdir -p keys
if [ ! -f keys/private.pem ]; then
    openssl genpkey -algorithm RSA -out keys/private.pem -pkeyopt rsa_keygen_bits:2048 2>/dev/null
    openssl rsa -in keys/private.pem -pubout -out keys/public.pem 2>/dev/null
    chmod 600 keys/private.pem
    echo "JWT keys generated."
else
    echo "JWT keys already exist."
fi

# 5. Generate secrets
echo "[5/7] Generating secrets..."
MASTER_KEY=$(openssl rand -base64 32)
PG_PASS=$(openssl rand -hex 16)
MINIO_PASS=$(openssl rand -hex 16)

# 6. Create .env
echo "[6/7] Creating .env..."
if [ ! -f .env ]; then
    cat > .env << EOF
# === Valt Production Environment ===
# Generated on $(date -u +%Y-%m-%dT%H:%M:%SZ)

# --- Database ---
POSTGRES_USER=valtag
POSTGRES_PASSWORD=${PG_PASS}
POSTGRES_DB=valtag

# --- MinIO (Object Storage) ---
MINIO_ROOT_USER=valtminio
MINIO_ROOT_PASSWORD=${MINIO_PASS}
MINIO_BUCKET=valtag-secrets

# --- Encryption ---
VAULT_MASTER_KEY=${MASTER_KEY}

# --- Domain (leave empty for IP-only) ---
DOMAIN=
ACME_EMAIL=

# --- URLs ---
GOOGLE_REDIRECT_URL=http://103.142.137.232/api/v1/auth/google/callback
DASHBOARD_URL=http://103.142.137.232
BASE_URL=http://103.142.137.232

# --- Google OAuth (optional — fill in after creating credentials) ---
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

# --- Stripe Billing (optional) ---
STRIPE_SECRET_KEY=
STRIPE_WEBHOOK_SECRET=
STRIPE_PRO_PRICE_ID=
STRIPE_TEAM_PRICE_ID=

# --- Slack (optional) ---
SLACK_BOT_TOKEN=
SLACK_SIGNING_SECRET=
SLACK_CLIENT_ID=
SLACK_CLIENT_SECRET=

# --- Telegram (optional) ---
TELEGRAM_BOT_TOKEN=
TELEGRAM_BOT_USERNAME=valtbot

# --- Zalo (optional) ---
ZALO_OA_TOKEN=
ZALO_OA_ID=

# --- SMTP Email (optional) ---
SMTP_HOST=
SMTP_PORT=587
SMTP_USER=
SMTP_PASSWORD=
SMTP_FROM=noreply@valt.dev

# --- Redis (optional) ---
REDIS_URL=
EOF
    echo ".env created with auto-generated secrets."
else
    echo ".env already exists — skipping."
fi

# 7. Summary
echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "  1. Copy your project files to /opt/valtag/"
echo "     scp -r . root@103.142.137.232:/opt/valtag/"
echo ""
echo "  2. SSH in and start services:"
echo "     cd /opt/valtag"
echo "     docker compose -f docker-compose.prod.yml up -d --build"
echo ""
echo "  3. Check status:"
echo "     docker compose -f docker-compose.prod.yml ps"
echo "     docker compose -f docker-compose.prod.yml logs -f server"
echo ""
echo "  4. Access:"
echo "     Dashboard: http://103.142.137.232"
echo "     API:       http://103.142.137.232/api/v1/health"
echo ""
echo "=== Generated Secrets (SAVE THESE) ==="
echo "VAULT_MASTER_KEY: ${MASTER_KEY}"
echo "POSTGRES_PASSWORD: ${PG_PASS}"
echo "MINIO_ROOT_PASSWORD: ${MINIO_PASS}"
echo ""
