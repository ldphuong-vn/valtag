# Positioning & Product Thesis (internal)

> Chốt ngày 2026-11-01, sau graphify toàn repo + research đối thủ. Đây là tài liệu định vị chính thức cho đến khi bị thay thế. Ngữ cảnh chi tiết: đợt research 11/2026 (Infisical Agent Vault docs, OneCLI repo+site, Gravitee/Aembit blogs, GitHub API momentum).

## 1. Một câu định vị

**"The ask-first credential gate for the agents you already have."**

Agent xin → người duyệt → lease tự hết hạn → audit hash-chain verify được. Giữ nguyên Claude Code / Cursor / ZCode / CI của bạn — chỉ cắm 1 MCP server + 1 URL. Self-host, Apache-2.0, 1 Go binary + Postgres (lưu cả blob mã hóa — MinIO bị archive 25/4/2026, tháo khỏi stack trong Phase 0).

## 2. USP — 4 trụ cột (đã verify đối thủ không có combo này)

1. **Approval workflow là dịch vụ độc lập** — không yêu cầu adopt harness nào. OneCLI có approval nhưng chỉ cho agent trong sandbox của họ; Infisical Agent Vault brokering thầm lặng (không human-in-loop); Aembit cố tình bỏ human-in-loop; Gravitee chỉ elicitation 1 bước.
2. **Credential lifecycle công bố rõ**: lease TTL (cap 1h), single-use, revoke cascade. OneCLI/Infisical AV không công bố TTL/lease.
3. **Audit tamper-evident**: hash chain + verify endpoint + off-site anchoring. Không đối thủ nào trong nhóm agent-space có.
4. **Approval → dynamic mint coupling**: duyệt xong mới mint (temp PG role, derived key) rồi tự thu hồi. OpenBao/Vault có dynamic secrets nhưng tách rời workflow, không MCP.

## 3. Bản đồ đối thủ (momentum 2026-11-01)

| Đối thủ | Momentum | Mô hình | Khoảng trống họ bỏ lại |
|---|---|---|---|
| OneCLI (YC) | 3.6k⭐, tạo 2026-03-08, push hàng ngày | Platform: sandboxed agent harness, approval trong Slack, identity gateway MITM | Phải adopt harness; không công bố lease/TTL; không hash-chain |
| Infisical | 30.2k⭐ | Secrets platform + Agent Vault (proxy gắn credential, sessions) | Không human-approval trong AV; không audit chain; license không thuần OSS |
| 1Password | Pivot 5/2026 | Service accounts + CLI cho agent infra | Proprietary, cloud-first, không self-host |
| Gravitee | Commercial | MCP Server Mediation: OAuth token exchange + elicitation | Mediation là phần thương mại; consent 1 bước |
| Aembit | Commercial SaaS | Workload IAM: attestation + JIT injection | Không human-in-loop (by design) |
| Higress | 9.7k⭐, CNCF | AI gateway host MCP + marketplace, auth plugin | Không vault, không approval, không audit — đối tác tiềm năng, không đối thủ |
| OpenBao/Vault | 8.8k⭐ / incumbent | Dynamic secrets engines | Không MCP, không agent workflow, ops nặng |

**Bài học OneCLI:** cùng ý tưởng, khởi đầu cùng thời điểm (3/2026), họ 3.6k⭐ với YC funding vs mình 0 release solo → **không đua execution tổng thể, đua khe hẹp + đúng timing.**

## 4. Luật chơi (do/don't)

**DO**
- Nhắm người dùng có agent sẵn: Claude Code/Cursor/ZCode users, CI, cron.
- Wedge distribution: plugin credential-source cho AI gateways (Higress/Kong) — họ thiếu đúng "bộ não" này.
- Dynamic backends mở rộng (AWS STS, GCP, GitHub App token, OpenAI scoped key) — làm roadmap, coupling với approval.
- Nội dung: "Your agents don't need a new home — they need a gate" (đâm platform-capture của OneCLI + silent-brokering của Infisical AV).
- Tuân thủ MCP OAuth 2.1 / RFC 8693 khi chuẩn hoá để tích hợp mọi gateway.

**DON'T**
- Không build agent platform (sandbox/runner/skills/channels) — đó là sân của OneCLI.
- Không đua độ rộng engines với Vault/OpenBao/Infisical.
- Không kể chuyện security khi chưa vá: **Phase 0 là điều kiện tiên quyết** — Telegram webhook approve chưa xác thực (critical), 6 endpoint BOLA (org members, agent tokens, audit/logs, webhooks, usage, slack integration), revoke-all chưa giết human sessions, master key rotation chưa có.
- Không claim "zero-knowledge" (server decrypt khi giao lease) — nói "least-knowledge".

## 5. Điều kiện tiên quyết trước khi marketing

1. Phase 0 hardening (~2 tuần): vá critical + BOLA, revoke-all đủ sâu, contract fixes (by-secret endpoint, name filter), CI có Postgres cho integration tests.
2. Tag v0.1.0 đầu tiên + release thật (CLI + MCP pipelines có sẵn, chưa từng chạy).
3. Prod topology TLS đúng (Caddyfile.prod thiếu `/api/*` branch).

## 6. Naming — ĐÃ CHỐT `valtag` (2026-11-02)

Tên chính thức: **valtag** — viết ngắn của *"valtag's agents"*, đồng thời gợi hành vi gắn thẻ/lease lên agent. `valtag-ng` bị loại: "ng" là hậu tố kỹ thuật cũ, khó đọc quốc tế.

Collision check 2026-11-02 (RDAP + registry chính thức): npm, PyPI, crates.io trống; GitHub 0 repo liên quan (1 hit `ValtaGamesProject` không liên quan); domain `valtag.dev/.io/.sh` đều trống.

**Kế hoạch:** giữ brand "valtag" trong code/UI đến khi chạy rename trong 1 session duy nhất — đặt **ngay trước tag v0.1.0** (0 tag/0 release/0 user = cửa sổ rename miễn phí cuối cùng). Checklist: `docs/rename-plan.md`. Việc kèm khi rename: tạo org GitHub `valtag-dev`, đăng ký domain sớm, check nốt `valtag.net/.org`.
