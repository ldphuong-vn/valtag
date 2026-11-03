# Deployment Guide

## Development

```bash
# Setup
git clone https://github.com/ldphuong-vn/valtag.git
cd valt
bash scripts/setup-dev.sh

# Or manually:
cp .env.example .env
docker compose up -d --build
make migrate-up
make seed
```

### Access Points
| Service | URL |
|---------|-----|
| API | http://localhost:8080 |
| Dashboard | http://localhost:3000 |
| Proxy | http://localhost:8443 |
| MinIO Console | http://localhost:9001 |
| PostgreSQL | localhost:5432 |

## Production (Single VPS)

### Prerequisites
- VPS: 8 vCPU, 16GB RAM, 200GB SSD
- Domain: valt.dev (or similar)
- DNS: api.valt.dev + app.valt.dev → VPS IP
- SMTP credentials

### Deploy
```bash
git clone https://github.com/ldphuong-vn/valtag.git
cd valt
cp .env.example .env.prod  # configure production values
docker compose -f docker-compose.prod.yml up -d
```

Caddy handles TLS automatically via Let's Encrypt.

### Verify
```bash
curl https://api.valt.dev/health
```
