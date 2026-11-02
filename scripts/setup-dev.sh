#!/usr/bin/env bash
set -euo pipefail

echo "=== Valt Dev Environment Setup ==="

# Copy .env if not exists
if [ ! -f .env ]; then
    cp .env.example .env
    echo "[ok] Created .env from .env.example"
else
    echo "[skip] .env already exists"
fi

# Generate JWT RS256 key pair if not exists
if [ ! -f keys/private.pem ]; then
    mkdir -p keys
    openssl genpkey -algorithm RSA -out keys/private.pem -pkeyopt rsa_keygen_bits:2048 2>/dev/null
    openssl rsa -pubout -in keys/private.pem -out keys/public.pem 2>/dev/null
    echo "[ok] Generated JWT RS256 key pair in keys/"
else
    echo "[skip] JWT keys already exist"
fi

# Start services
echo ""
echo "Starting Docker services..."
docker compose up -d --build

# Wait for postgres
echo "Waiting for PostgreSQL..."
until docker compose exec -T postgres pg_isready -U "${POSTGRES_USER:-valtag}" > /dev/null 2>&1; do
    sleep 1
done
echo "[ok] PostgreSQL ready"

echo ""
echo "=== Setup Complete ==="
echo "  API:       http://localhost:8080/health"
echo "  Dashboard: http://localhost:3000"
echo "  Proxy:     http://localhost:8443"
echo "  MinIO:     http://localhost:9001"
echo "  PostgreSQL: localhost:5432"
echo ""
echo "Next steps:"
echo "  make migrate-up  # Run database migrations"
echo "  make seed         # Seed development data"
