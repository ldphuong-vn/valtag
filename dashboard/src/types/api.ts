export interface Secret {
  id: string
  name: string
  description: string
  credential_type: string
  source: string
  status: string
  owner_id: string
  created_at: string
  updated_at: string
}
export type {
  PolicyParameters,
  PolicyTemplate,
  PolicyTemplateVersion,
  PolicyTemplateRef,
  SecretPolicyBinding,
} from './policy'

export interface AccessRequest {
  id: string
  secret_id: string
  secret_name?: string
  requester_user_id: string
  requester_type: string
  ai_agent_id?: string
  reason?: string
  requested_duration_minutes: number
  status: 'pending' | 'approved' | 'rejected' | 'expired' | 'revoked'
  decided_by?: string
  decided_at?: string
  expires_at?: string
  rejection_reason?: string
  created_at: string
}

export interface Credential {
  id: string
  access_request_id: string
  credential_type?: string
  status: string
  expires_at: string
  usage_count: number
  revoked_at?: string
  created_at: string
  value?: string
}

export interface AuditLog {
  id: string
  user_id: string
  action: string
  resource_type: string
  resource_id: string
  event_time: string
  event_type: string
  status: string
  ip_address: string
  metadata: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  limit: number
}

export interface User {
  id: string
  email: string
  display_name?: string
  avatar_url?: string
  region_code: string
  auth_provider: string
}

export interface Organization {
  id: string
  name: string
  slug: string
  owner_id: string
  created_at: string
  updated_at: string
  // mapping visibility: org → workspace → project chain (2026-11-21)
  workspace_count?: number
  project_count?: number
}

export interface Workspace {
  id: string
  org_id: string
  name: string
  slug: string
  created_at: string
  updated_at: string
}

export interface Project {
  id: string
  workspace_id: string
  name: string
  slug: string
  created_at: string
  updated_at: string
}

export interface OrgMembership {
  org_id: string
  user_id: string
  role: string
  created_at: string
}

export interface ProjectMembership {
  project_id: string
  user_id: string
  role: string
  permissions: Record<string, unknown>
  created_at: string
}

export interface AgentIdentity {
  id: string
  project_id: string
  name: string
  description: string
  agent_type: string
  auth_method: string
  allowed_scopes: string[]
  max_session_ttl: number
  status: string
  created_by: string
  created_at: string
  updated_at: string
}

export interface AgentToken {
  id: string
  agent_id: string
  scopes: string[]
  expires_at: string | null
  last_used_at: string | null
  created_at: string
  revoked_at: string | null
  token?: string  // only present on creation
}

export interface ScanResult {
  id: string
  project_id: string
  scanned_by: string
  scan_path: string
  findings_count: number
  status: string
  created_at: string
}

export interface DynamicProvider {
  id: string
  project_id: string
  name: string
  provider_type: string
  status: string
  created_by: string
  created_at: string
  updated_at: string
}

export interface DynamicLease {
  id: string
  provider_id: string
  agent_id?: string
  credentials?: Record<string, string> // only on creation
  ttl_seconds: number
  expires_at: string
  revoked_at?: string
  created_at: string
}

export interface ScanFinding {
  id: string
  scan_id: string
  file_path: string
  line_number: number
  pattern_name: string
  credential_type: string
  redacted_preview: string
  status: string
  imported_secret_id?: string
  created_at: string
}

export interface NotificationChannel {
  id: string
  user_id: string
  channel_type: 'slack' | 'telegram' | 'email'
  handle: string
  verified: boolean
}

export interface ProxyRoute {
  id: string
  agent_id: string
  host_pattern: string
  path_pattern: string
  secret_id: string
  injection_type: string
  injection_key: string
  injection_format: string
  placeholder_key: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface EndpointLimit {
  id: string
  agent_id: string
  host_pattern: string
  path_pattern: string
  rpm: number
  blocked: boolean
  created_at: string
}
