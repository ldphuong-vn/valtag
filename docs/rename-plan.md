# Rename Plan — valtag → **valtag** (đã chốt 2026-11-02)

> Trạng thái 2026-11-02: **ĐÃ CHỐT tên `valtag`** — viết ngắn của *"valt's agents"*. Collision check 2026-11-02: npm / PyPI / crates.io trống; GitHub sạch; domain `valtag.dev/.io/.sh` trống. Giữ brand "valtag" trong giai đoạn hardening; chạy checklist này trong **một session duy nhất, ngay trước tag v0.1.0** (0 tag/0 release = cửa sổ rename miễn phí cuối cùng).

## 1. Memo quyết định tên

| Lựa chọn | Ưu | Nhược | Collision đã check (11/2026) |
|---|---|---|---|
| **valt** (giữ) | Chi phí 0; CLI/prefix/env giữ nguyên; org `valt-dev` + `valt.dev` còn trống để nhận | Ý nghĩa "vault" ngược định vị "gate"; npm `valt` đã bị chiếm; dễ đọc nhầm là HashiCorp Vault clone | GitHub: `btcdoomguy/valt` (nhỏ). npm: taken. PyPI: free. |
| **valtag** ✅ CHỐT 2026-11-02 | Dễ đọc, gợi "tag/hạn mức", giữ dòng họ valtag; viết ngắn của "valt's agents" | Vẫn "vault-ish"; dài hơn | **Sạch 2026-11-02**: npm/PyPI/crates trống; GitHub 0 hit liên quan; valtag.dev/.io/.sh trống |
| ~~valt-ng~~ (loại) | Đánh dấu generation mới sau refactor | "ng" là hậu tố kỹ thuật lỗi thời (angular-vibes), khó phát âm quốc tế | Sạch (2026-11-02) nhưng bị loại vì công dụng |
| askfirst | Tên = hành vi sản phẩm; namespace sạch nhất (PyPI + domain trống, collision duy nhất tool R nhỏ) | Bỏ hoàn toàn brand "valtag"; phải rename toàn bộ | Đã check sạch (11/2026) |
| gatehouse | Metaphor cổng có người gác, chuyên nghiệp | Dài cho binary | vài repo nhỏ khác ngành; gatehouse.dev không phản hồi |

**Lưu ý trước khi chốt:** (1) đổi prefix token (`valtag_pk_`, `valtag_dk_`) phá mọi lease/key đang lưu trong DB — cần migration nghĩ kỹ hoặc giữ prefix cũ làm legacy; (2) đổi config dir `~/.valt` + keychain service `valt` phải có path migration từ cài đặt cũ; (3) nếu rename GitHub repo, GitHub tự redirect URL cũ nhưng Go module path KHÔNG redirect — phải đổi cùng lúc `go.mod` + toàn bộ import.

## 2. Checklist cơ học (chạy khi đã chốt)

Thứ tự an toàn: tạo org/repo mới trên GitHub trước → clone mới → chạy các bước dưới → CI xanh → rồi mới rename repo cũ thành redirect.

```bash
# 0. Biến toàn cục (điền tên mới)
OLD=valt   # NEW=valtag

# 1. Go server module + mọi import (server/ gồm cmd/valt CLI#2 — nếu chưa xóa CLI#2 thì xóa trước)
grep -rl "github.com/ldphuong-vn/valtag/server" server --include="*.go" | xargs sed -i "s|github.com/ldphuong-vn/valtag/server|github.com/ldphuong-vn/valtagag/server|g"

# 2. valt-cli: module + config dir + keychain service + binary
grep -rl "valt" valt-cli --include="*.go"   # review tay trước khi sed hàng loạt
#   - ~/.valt/config.toml  →  ~/.<new>/config.toml      (valt-cli/internal/config/config.go)
#   - keychain service "valt" / legacy "valt-cli"       (valt-cli/internal/keychain/keychain.go)

# 3. mcp-server (Rust): binary name, keychain entries, env var names
#   - Cargo.toml package/bin name valtag-mcp-server
#   - keychain service "valt-agent"/"valt"              (mcp-server/src/keychain.rs)
#   - env: VALTAG_AGENT_TOKEN, VALT_TOKEN                 (mcp-server/src/client.rs)

# 4. Token prefixes (NGUY HIỂM — phá DB hiện có, cần quyết legacy):
#   - valtag_dk_  (server/internal/dynsecret/derived_key.go:15)
#   - valtag_pk_  (server/internal/gateway/injector.go — placeholder scan)
#   Đề xuất: giữ prefix cũ làm legacy-accepted + mint prefix mới, đọc cả hai trong 2 version.

# 5. Env vars server (chỉ đổi khi chấp nhận break deploy cũ):
#   VAULT_MASTER_KEY, JWT_*, GATEWAY_ENABLED ... (server/internal/config/config.go)
#   + docker-compose*.yml, .env.example, .env.production.example

# 6. Dashboard: tiêu đề/branding, login copy, README badge URL
# 7. Docs: README.md, docs/*.md, CLAUDE.md, SECURITY.md, Makefile, .github/workflows/*
# 8. Rename GitHub repo (Settings → General) + tạo org mới nếu dùng org path
# 9. Domain: register <new>.dev/.sh nếu cần; update CLI default URL (valt-cli/internal/config/config.go:17) và MCP default (mcp-server/src/config.rs:14-16) — hiện 3 client đang có 3 default khác nhau, nhân tiện thống nhất.

## 3. Việc làm kèm lúc rename (dọn dẹp phải làm sẵn trước, xem graphify report)
- [ ] Xóa `server/cmd/valt` (CLI#2: run là stub, gửi `plaintext_value` sai contract) — chỉ giữ valt-cli/
- [ ] Xóa hoặc wire: webhooks dispatcher, usage.EnforceLimits, agent.AuthMiddleware, approval-chain multi-step, consent
- [ ] Xóa root clutter: note.md, idea-saas-valt.md, 2 e2e script trùng, plans/ (hoặc move ra repo riêng)
- [ ] Sửa `make test-integration` trỏ đúng thư mục test thật (package dirs, không phải server/tests/)
