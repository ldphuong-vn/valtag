// Must match the server's validCredentialTypes (server/internal/vault/handler.go).
// tls_cert was removed: the server never accepted it (dogfood 2026-11-13).
export const CREDENTIAL_TYPES = [
  { value: 'api_key', label: 'API Key' },
  { value: 'oauth_token', label: 'OAuth Token' },
  { value: 'db_credential', label: 'Database Credential' },
  { value: 'ssh_key', label: 'SSH Key' },
  { value: 'cloud_credential', label: 'Cloud Credential' },
  { value: 'personal_session', label: 'Personal Session' },
] as const

export const REQUEST_STATUSES = {
  pending: { label: 'Pending', color: 'yellow' },
  approved: { label: 'Approved', color: 'green' },
  rejected: { label: 'Rejected', color: 'red' },
  expired: { label: 'Expired', color: 'gray' },
  revoked: { label: 'Revoked', color: 'orange' },
} as const

export const BACKEND_URL = process.env.NEXT_PUBLIC_BACKEND_URL ?? 'http://localhost:8080'
