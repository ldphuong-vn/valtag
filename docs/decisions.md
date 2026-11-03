# valtag — Nhật ký quyết định chiến lược (2026-08-31)

Tài liệu này ghi lại các quyết định định hướng cho valtag và **lý do** đưa ra quyết định
đó, để tránh tranh luận lại từ đầu mỗi lần phát triển. Cập nhật khi có quyết định mới.

---

## Bối cảnh

- valtag khởi động 2026-03-17 với ý tưởng "MCP-native secret vault + human-in-the-loop
  approval cho AI agent". Sau 10 ngày phát triển dồn dập (đến 2026-03-26) có SaaS layer,
  admin dashboard, rồi dừng từ đó đến nay (0 star, 0 release).
- Đối thủ tham chiếu: [onecli/onecli](https://github.com/onecli/onecli) — khởi động
  **sớm 9 ngày** cùng ý tưởng gốc ("credential vault cho AI agent viết bằng Rust"),
  đã pivot thành agent harness cho team: 3.430 star, release 2–3 ngày/lần, core
  Apache-2.0 + thư mục `ee/` chạy Enterprise License (khóa SSO, SCIM, RBAC chi tiết,
  KMS, HA, budget, multi-workspace).

---

## Quyết định 1: KHÔNG làm SaaS. Thuần tự host, thuần Apache-2.0

**Quyết định:** valtag không bán SaaS, không có pricing/plan/enterprise license.
Mọi tính năng đều nằm trong bản tự host, Apache-2.0 toàn bộ, không `ee/`, không CLA.

**Lý do:**
- Sản phẩm bảo mật dạng SaaS rất khó tạo trust với người dùng; trust trong self-hosted
  đến từ **code đọc được**, không phải thương hiệu — valtag nhỏ đến mức một công ty
  audit được trong một tuần (221 file Go + MCP server Rust, so với 2.200 file của onecli).
- Chính onecli là bằng chứng mô hình open-core kẹp người dùng: `ee/` khóa đúng những
  thứ doanh nghiệp tự host cần nhất (SSO, RBAC). CLA của họ cho phép relicense core
  bất cứ lúc nào — "Apache hôm nay" không phải cam kết vĩnh viễn.
- **Vị trí cạnh tranh:** valtag là lựa chọn mà onecli về cấu trúc không thể là:
  nhỏ, thuần Apache mãi mãi, không khóa dần. Câu này là vũ khí marketing, không phải
  chỉ lý tưởng.

**Hệ quả:** cắt landing pricing, upgrade page, SaaS super-admin dashboard, billing.

## Quyết định 2: Định vị "MCP credential gate" nhẹ — KHÔNG đối đầu harness

**Quyết định:** valtag là lớp phát hành & quản trị quyền (entitlement plane) giữa
AI gateway/hạ tầng và các agent runtime có sẵn. Không làm sandbox, runner,
gateway MITM, jumpserver UI.

**Lý do:**
- onecli đã có approval + secret store ở **core miễn phí** — làm lại hai thứ đó
  tốt hơn 20% là thua. Điểm yếu của họ: bắt buộc vào hệ sinh thái (6 service,
  agent phải chạy trong sandbox của họ). Người đã có agent runtime (Claude Code,
  Cursor, framework riêng) chỉ cần một lớp credential mỏng — đó là kẽ hở của valtag.
- Kịch bản neo: **doanh nghiệp cấp AI API cho nhân viên như cấp laptop.**
  Map khái niệm: laptop có serial/MDM → agent có identity; admin password không phát
  cho user → master key không phát cho nhân viên, chỉ phát lease ngắn hạn;
  biên bản sử dụng → audit hash-chain. valtag đứng đúng ranh giới
  "IdP nói người này là ai — gateway đo họ dùng bao nhiêu — **valtag nói họ được cầm
  key nào, bao lâu, ai duyệt, vết audit không sửa được**".

## Quyết định 3: Key features = dynamic lease + revoke cascade + audit 4W1H

**Quyết định:** Ba tính năng chủ lực, theo thứ tự:
1. **Dynamic lease** (phát hành credential ngắn hạn dẫn xuất; nối quy trình
   approval → lease — hiện hai module đang rời nhau).
2. **Revoke cascade theo người** (nhân viên nghỉ → một lệnh cắt mọi token/agent/lease).
3. **Audit 4W1H đầy đủ + hash-chain đúng nghĩa + verify/export** (who/what/when/
   where/why/how; where = IP + hostname agent; how = chuỗi hash chống sửa).

**Lý do:** Lease ngắn hạn chỉ có nghĩa khi thu hồi được ngay (IdP deactivate →
mọi lease chết trong vài giây). Audit hash-chain là khác biệt lớn nhất so với
onecli (họ khóa audit review vào enterprise) — nhưng hiện tại code có bug khiến
claim đó chưa đúng (xem refactor-plan.md §Audit). Thứ tự xếp theo dependency:
sửa nền tin cậy trước, nối tính năng sau.

## Quyết định 4: KHÔNG làm IPAM, KHÔNG tự build jumpserver

**Quyết định:** Không đụng IPAM. Không xây jumpserver. Mở rộng hạ tầng sau này
(chỉ khi có nhu cầu thật) bằng **chứng chỉ ngắn hạn** (SSH cert do valtag ký CA),
tái dùng nguyên quy trình duyệt + audit — không thay thế hạ tầng truy cập có sẵn.

**Lý do:** IPAM thuộc lớp network infrastructure, không có synergy với lớp
credential. Jumpserver là cùng bài toán của valtag nhưng áp lên hạ tầng — đúng cách
là valtag thành nơi cấp quyền cho hệ thống truy cập có sẵn (Teleport/Boundary/bastion),
không phải làm thêm một sản phẩm. Tự build mọi láng giềng gần là con đường onecli
nổ phình scope (2.200 file, 6 service).

## Quyết định 5: Đúng 2 client — CLI (Go) là cửa vào duy nhất + MCP server (Rust)

**Quyết định:** Không thêm client mới nào. Giữ `valt-cli` (Go) làm điểm vào:
`valtag setup` → `valtag mcp install --ide claude` tự cài MCP server. Rust MCP server
là client thật của các AI tool. Phát hành binary qua GitHub Releases + CI
(cross-platform Windows/macOS/Linux, có checksum), **không bao giờ commit binary
vào repo** (đã dọn 36MB .exe hôm 2026-08-31 và thêm rule .gitignore).

**Lý do:** Người dùng thật của credential là AI tool (→ MCP server); người cần
một lệnh duy nhất để bắt đầu là nhân viên (→ CLI kiêm installer). Reproducible
build + single binary là một trụ của trust-by-code.

## Quyết định 6: Tài khoản repo

**Quyết định (chưa thi hành):** khi tiếp tục phát triển chính thức, **transfer**
repo `trablob410/vaultsaas` → `ldphuong-vn` (hoặc org riêng) thay vì clone làm repo
mới — giữ issues/PR/releases/secrets/CI nguyên vẹn, GitHub tự redirect URL cũ.

**Lý do:** clone mới mất metadata và cấu hình secrets, để lại hai repo chồng chéo.

---

## Nguyên tắc vận hành rút ra từ so sánh onecli

- **Bền vững > tính năng:** cùng xuất phát điểm, khác biệt của onecli là kỷ luật
  release 2–3 ngày/lần trong 5 tháng. Nếu không duy trì được nhịp đều đặn
  (~10–15h/tuần), cắt tham vọng chứ không để repo chết dần.
- **Trust bằng code, không bằng marketing:** codebase nhỏ audit được, reproducible
  build, audit tự verify được. Không claim gì code chưa làm được (đang còn claim
  "zero-knowledge" trong README — server hiện decrypt rồi trả plaintext → phải sửa).

## Thư mục `plans/` — GIỮ NGUYÊN

Là tài liệu tham khảo nội bộ của các đợt phát triển trước; giữ làm docs tra cứu.
