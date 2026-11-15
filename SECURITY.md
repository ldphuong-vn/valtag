# Security Policy

## Reporting a vulnerability

**Preferred: GitHub Private Vulnerability Reporting** — use the
"Report a vulnerability" button on the **Security** tab of this repository.
No email, no forms, goes straight to the maintainer privately.

If you prefer email: security@valt.dev (monitored).

Please include:
- The affected component (server / dashboard / MCP server / gateway listener)
- Steps to reproduce or a proof of concept
- Potential impact
- A suggested fix, if you have one

**What to expect**: acknowledgment within 48 hours, a fix timeline within 7
days, and credit in the release notes (unless you prefer to stay anonymous).

## Safe harbor

Good-faith security research is welcome and will not be met with legal
action, provided you:

- Research **against your own self-hosted deployment** — not against someone
  else's, and not against public demo instances without coordination
- Avoid privacy violations, data destruction and service degradation
  (no DoS, no spam, no social engineering)
- Give the maintainer a reasonable window to ship a fix before any public
  disclosure (90 days is the standard we commit to)

## Security contributions are welcome — explicitly

valtag is a credential gate: its value is exactly as strong as its weakest
security path. Beyond vulnerability reports, the project actively welcomes:

- **Code review of the security-critical paths** — the auth/JWT layer, the
  approval state machine (`workflow/`), the derived-key validation
  (`dynsecret/`), the gateway authorize endpoint (`gateway/`), and the audit
  hash chain (`audit/`). If you can break the chain or the lease model, that
  is a headline-worthy finding and we will credit it as one.
- **Threat-model reviews** of [docs/security-model.md](docs/security-model.md)
  and [docs/ax.md](docs/ax.md) — the gateway authorize contract is new and
  externally reviewable.
- **Hardening PRs** — rate limiting, session handling, secret-scanner
  coverage, fail-closed paths.

Open a normal PR or a GitHub Discussion (category *Show and tell* or
*Ideas*); security-relevant changes get maintainer review priority.

**No bug bounty**: valtag is solo-maintained Apache-2.0 — there is no bounty
budget. What we offer instead: fast fixes, public credit, and a project that
treats security review as a feature, not a liability.

## Scope

**In scope**: the Go server, the Next.js dashboard, the Rust MCP server, the
gateway listener, and the Compose deployments in this repository — tested
against your own self-hosted instance.

**Out of scope**: social engineering, physical attacks, denial of service,
spam, and attacks against third-party services (demo domains, DNS, email).
Reports about the shared staging demo should be coordinated first — see
[docs/integrations/](docs/integrations/README.md) for what runs there.

## Security model (summary)

valtag is **least-knowledge, not zero-knowledge** — stated honestly:

- Secret values are encrypted client-side (AES-256-GCM envelope encryption:
  DEK wraps the value, master key wraps the DEK); the server stores
  ciphertext. **The server does decrypt approved secrets to deliver them** —
  plaintext exists only at approved delivery time.
- Credentials issued to agents are leases with TTLs; derived keys
  (`valtag_dk_…`) are HMAC-bound and useless outside valtag's gateway.
- The audit log is a SHA-256 hash chain — editing any row breaks verification;
  anchors can be kept off-site to detect truncation.

See [docs/security-model.md](docs/security-model.md) for the full model and
[docs/ax.md](docs/ax.md) for the gateway authorize contract.
