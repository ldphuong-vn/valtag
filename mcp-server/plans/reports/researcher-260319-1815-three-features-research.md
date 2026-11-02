# Research Report: Three valtag Features
**Date:** 2026-03-19 | **Researcher:** claude-haiku

---

## 1. TOTP 2FA in Go

**Library:** `pquerna/otp` ✓ RECOMMENDED
- **Status:** Maintained (Dec 2024 release). 1,333+ known importers. Apache-2.0 licensed.
- **Key Functions:**
  - `totp.Generate(account)` → creates secret + QR-code-capable key
  - `key.Image(200, 200)` → PNG bytes for QR display (or base64-encoded for frontend)
  - `totp.Validate(code, secret)` → verify user entry (built-in TOTP window tolerance)
  - `GenerateCodeCustom(secret, time.Now(), opts)` → for tests/custom windows

**DB Schema Required:**
```sql
ALTER TABLE users ADD COLUMN totp_secret VARCHAR(32) NULL;
ALTER TABLE users ADD COLUMN totp_enabled BOOLEAN DEFAULT FALSE;
CREATE TABLE backup_codes (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash VARCHAR(255) NOT NULL,  -- bcrypt hash of code
  used_at TIMESTAMP NULL,
  created_at TIMESTAMP DEFAULT NOW()
);
```

**Implementation Flow:**
1. **Setup:** Generate secret → store in session (not DB yet) → display QR + secret text
2. **Verify:** User scans + enters code → `totp.Validate(code, secret)` → if valid, save `totp_secret` to DB + generate 10 backup codes (hashed)
3. **Login:** On password auth, check `totp_enabled` → demand TOTP code + validate → if failed 3x, lock account 15min
4. **Recovery:** If user loses phone, accept one backup code (mark used), prompt regenerate backup codes or contact support

**Security Notes:**
- Store `totp_secret` plaintext in DB (can't recover if lost; users have backup codes for recovery)
- Backup code hashes with bcrypt, salt cost ≥12
- Rate-limit code verification (max 3 wrong attempts per 5min)
- Warn user during setup: "Save backup codes immediately; losing both phone + codes = account lockout"

---

## 2. Shamir's Secret Sharing for Master Key Recovery

**Library:** `github.com/hashicorp/vault/shamir` ✓ RECOMMENDED
- **Status:** Production-grade (used in HashiCorp Vault). Alternative: `corvus-ch/shamir` (community fork).
- **Core API:**
  - `Split(secret []byte, shares, threshold int) ([][]byte, error)` → N shares, need M to reconstruct
  - `Combine(shares [][]byte) ([]byte, error)` → reconstruct secret from M shares

**Recommended Config:** Split VAULT_MASTER_KEY into **5 shares, threshold 3** (any 3 unlock the key)
- 2 user-downloaded shares (PDF, printed + stored securely)
- 2 backups (encrypted in separate geographic S3 bucket, different AWS account)
- 1 server-side (air-gapped HSM, not general server)

**Implementation Approach:**
1. **At Setup:** Key generation service (off-box) splits VAULT_MASTER_KEY into 5 shares → generates PDF for user download → logs retrieval event
2. **PDF Format:** QR code + plaintext + instructions: "Store offline. Print + laminate. Destroy digital copy."
3. **At Recovery:** Admin initiates recovery → collect 3 shares (e.g., user + 2 cloud backups) → `Combine()` → load into HSM/KMS
4. **UX:** Recovery is *intentionally* slow/multi-step (requires human approvals at each share location)

**DB Schema:** (New table only if tracking recovery events)
```sql
CREATE TABLE master_key_shares (
  id UUID PRIMARY KEY,
  share_index INT,  -- 0-4
  location VARCHAR(50),  -- "user", "aws-us-east", "aws-eu", "hsm", "backup-vault"
  encrypted_share BYTEA,  -- encrypted with transit key; never store plaintext
  last_verified TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW()
);
```

**Security Considerations:**
- **NEVER** store unencrypted shares on any single server. Each share location = different auth/infrastructure.
- User shares must be **offline.** Provide offline verification tool (standalone binary, no network).
- Server share must live in air-gapped HSM or offline vault. If compromised, 3-share threshold means attacker still needs 2 user shares.
- Recovery should require phone verification + time delays (24-48hr) to detect theft.

**Caveat:** This is *disaster recovery* only (e.g., root cause of outage destroys master key). Not for routine key rotation.

---

## 3. VSCode Extension for Secret Management

**Pattern:** Multi-part extension (TreeView sidebar + StatusBar + SecretStorage API)

**Core APIs (VSCode):**
- **TreeView + TreeDataProvider:** Hierarchical secret browser in sidebar. `getChildren()` fetches secrets; icons show status (pending approval, expired, healthy).
- **SecretStorage API:** `secretStorage.store(key, value)` — stores agent token securely (Keychain on macOS, Credential Manager on Windows, libsecret on Linux).
- **Authentication:** Use existing OAuth flow (if valtag supports OAuth) OR reuse agent token → store in SecretStorage → refresh on demand.
- **Command Palette:** `vault.requestSecret`, `vault.showApprovals`, `vault.injectEnv` (spawns terminal with `export` commands).

**Minimal Feature Set (MVP):**
1. **Auth:** Prompt for valtag URL + agent token on first install → store in SecretStorage
2. **TreeView Sidebar:**
   - List user's projects → expand to secrets
   - Icon: green = ready, orange = pending approval, red = expired
   - Click secret → show metadata (created, rotated, owner)
3. **Context Menu:**
   - "Copy secret value" → copies to clipboard (5-sec auto-clear)
   - "Request access" → opens browser to valtag approval UI
   - "Show approvals" → TreeView sub-panel listing pending requests
4. **Status Bar:** Show "valtag: 2 pending approvals" → click to expand approvals panel
5. **Command:** `valt-inject-env` → opens terminal, exports secrets as env vars

**Authentication Pattern:**
- **Reuse existing agent token** (simplest) — token must have `read:secrets` + `read:workflow` perms.
- On startup, validate token → if invalid, prompt to re-enter valtag URL + generate new token via CLI.
- Token stored in SecretStorage (never in workspace settings/git).

**Tech Stack:**
- **TypeScript** (strict mode) + Webview (TreeView only, no custom HTML)
- **Vite** or **esbuild** for bundling (fast builds)
- **@vscode/* dependencies only** (no extra runtimes)
- **API client:** Use existing Go valtag API (http client in TS via `node-fetch` or `axios`)

**Relative Priority:** Low-effort (TreeView + token storage) → Medium (approvals sidebar) → High-effort (intelligent env injection, secret rotation detection)

**Caveat:** VSCode extensions run in untrusted contexts. Never assume secrets are safe. Recommend:
- Secrets *always* expire in VSCode (max 1hr in memory)
- Flag to disable clipboard copy (require manual view-only)
- Log all fetches to valtag audit trail

---

## Key Insights

| Feature | Effort | Risk | ROI |
|---------|--------|------|-----|
| **TOTP 2FA** | ~1-2 sprints | Medium (backup code UX) | High (compliance, user trust) |
| **Shamir SSS** | ~2-3 sprints | High (complexity, testing) | Low-Medium (disaster recovery only) |
| **VSCode Ext** | ~1-2 sprints | Low (sandboxed) | Medium (developer convenience) |

---

## Unresolved Questions

1. **TOTP:** Should backup codes be regenerable, or one-time-only? (Regenerable = more UX burden; one-time = higher account-lockout risk.)
2. **Shamir:** Will valtag offer managed recovery (valtag holds 2 shares), or purely user-managed? Affects trust model.
3. **VSCode:** Should extension support secret *rotation* (auto-refresh env on file save), or view-only?
4. **VSCode:** Will valtag provide OAuth, or always agent-token-based auth for extensions?

---

## Sources

- [pquerna/otp GitHub](https://github.com/pquerna/otp)
- [otp/totp package (Go Packages)](https://pkg.go.dev/github.com/pquerna/otp/totp)
- [Shamir Secret Sharing - HashiCorp Vault](https://pkg.go.dev/github.com/hashicorp/vault/shamir)
- [Designing 2FA with Backup Codes (DEV Community)](https://dev.to/myougatheaxo/designing-2fa-totp-with-claude-code-google-authenticator-backup-codes-recovery-30ga)
- [TOTP Backup Codes - SuperTokens Docs](https://supertokens.com/docs/additional-verification/mfa/backup-codes)
- [Shamir Secret Sharing Best Practices (WebOfTrust)](https://github.com/WebOfTrustInfo/rwot8-barcelona/blob/master/draft-documents/shamir-secret-sharing-best-practices.md)
- [How to Share a Secret - MIT (Adi Shamir)](https://web.mit.edu/6.857/OldStuff/Fall03/ref/Shamir-HowToShareASecret.pdf)
- [VSCode API Reference](https://code.visualstudio.com/api/references/vscode-api)
- [HashiCorp Vault VSCode Extension](https://marketplace.visualstudio.com/items?itemName=owenfarrell.vscode-vault)
- [SecretStorage in VSCode Extensions (DEV Community)](https://dev.to/kompotkot/how-to-use-secretstorage-in-your-vscode-extensions-2hco)
- [Infisical Secret Management](https://infisical.com/)
