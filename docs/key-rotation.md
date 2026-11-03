# Key rotation & backup/restore — vận hành valtag

> Hướng dẫn vận hành cho self-host. Mọi lệnh đều chạy được với compose mặc định; các biến môi trường nằm trong `.env`.

## 1. Master key rotation

`VAULT_MASTER_KEY` mã hóa: secret DEKs (`secrets.encrypted_dek`), TOTP secrets (`users.totp_secret_enc`), provider configs (`dynamic_providers.config_enc`), lease credentials (`dynamic_leases.secret_data_enc`), integration tokens (`org_integrations.access_token_enc`).

Rotation = re-wrap toàn bộ các cột đó từ key cũ sang key mới, **trong một transaction** (thất bại ở bất kỳ dòng nào → rollback, DB không đổi).

### Khi nào cần rotate
- Key cũ bị lộ/khuất phục vụ;
- Chu kỳ bảo mật định kỳ (khuyến nghị 12 tháng);
- Rời khỏi nhân viên nắm key cũ.

### Các bước (server STOPPED)

```bash
# 0. Backup trước — luôn luôn (xem mục 2)
make backup

# 1. Dừng server (giữ Postgres chạy)
docker compose stop server

# 2. Tạo key mới + chạy rotation
export OLD_MASTER_KEY=$(grep '^VAULT_MASTER_KEY=' .env | cut -d= -f2)
export NEW_MASTER_KEY=$(openssl rand -base64 32)
make rotate-key

# 3. Đổi key trong .env rồi khởi động lại
sed -i "s|^VAULT_MASTER_KEY=.*|VAULT_MASTER_KEY=${NEW_MASTER_KEY}|" .env
docker compose up -d server
```

### Hệ quả cần biết
- **Derived API keys (`valtag_dk_*`) hết hiệu lực ngay** — chúng được HMAC từ master key. Agents re-request credentials (mô hình lease vốn vậy). Lease đang chạy chết cùng lúc.
- TOTP secrets, provider configs, integration tokens, secret values **sống sót nguyên vẹn**.
- Sự kiện `master_key.rotate` được ghi vào audit hash-chain (actor = nil UUID hệ thống).
- Chạy tool hai lần an toàn (idempotent về mặt kết quả: lần hai sẽ fail decrypt bằng OLD key nếu đã rotate — không đổi gì).

## 2. Backup & restore

Nhờ lưu blob trong Postgres (không còn object store riêng), **một `pg_dump` là toàn bộ trạng thái valtag** — secrets, leases, audit chain, configs.

### Backup

```bash
# Full logical backup (khuyến nghị hằng ngày qua cron)
docker compose exec postgres pg_dump -U valt -Fc valt > valtag-backup-$(date +%F).dump
```

Lưu file `.dump` ra nơi KHÁC máy hosting (object storage ngoài / máy khác) — backup cùng đĩa không phải backup. Giữ ≥ 30 ngày.

### Restore

```bash
# Vào một DB sạch (ví dụ sau sự cố hoặc migrate máy)
docker compose exec -T postgres psql -U valt -c "DROP DATABASE IF EXISTS valt; CREATE DATABASE valt;"
cat valtag-backup-2026-11-04.dump | docker compose exec -T postgres pg_restore -U valt -d valt --clean --if-exists
```

### Những gì backup KHÔNG chứa (lưu riêng)
- `VAULT_MASTER_KEY` — nằm trong `.env`. Mất key = mất toàn bộ secret values (chỉ còn ciphertext). Giữ key ở password manager/Vault riêng, KHÔNG để trong cùng backup Postgres.
- `keys/private.pem` + `keys/public.pem` (JWT signing) — mất = mọi phiên đăng nhập/CLI token vô hiệu (phải đăng nhập lại, không mất dữ liệu).
- `.env` còn lại (SMTP, OAuth, Telegram secrets).

**Công thức an toàn:** backup DB + backup `.env` + `keys/` ở **hai nơi khác nhau**. Cả ba cùng một nơi thì mất máy = mất tất cả.

## 3. Audit chain sau restore

Audit hash-chain sống trong chính DB được restore — chuỗi nguyên vẹn sau restore. Để chống **xóa đuôi** (ai đó restore một snapshot cũ để che sự việc), dùng off-site anchoring: định kỳ lưu `GET /api/v1/audit/verify` head hash ra nơi ngoài hệ thống (email/notes/object storage). Restore từ snapshot cũ sẽ lệch head → verify phát hiện.

```bash
# Lấy head hash hiện tại (cron hằng ngày, lưu off-site; cần JWT của admin)
curl -s -H "Authorization: Bearer $ADMIN_JWT" \
  "http://localhost:8080/api/v1/audit/verify" | jq .head
```
