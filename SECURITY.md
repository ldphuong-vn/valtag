# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in valtag, please report it responsibly.

**Email**: security@valt.dev

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

We will acknowledge receipt within 48 hours and provide a detailed response within 7 days.

## Security Model

valtag uses a zero-knowledge architecture:
- Secrets encrypted client-side with AES-256-GCM
- Envelope encryption (DEK wrapped by user master key)
- Server never sees plaintext secrets or master keys
- All access requires human approval
- Audit trail with hash chain integrity

See `docs/security-model.md` for details.
