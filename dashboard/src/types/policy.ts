export interface PolicyParameters {
  max_duration_minutes: number
  require_approval: boolean
  allow_auto_approve: boolean
  require_reason: boolean
  min_reason_length: number
  max_requests_per_day: number
  cool_down_minutes: number
  single_use: boolean
  notify_on_access: boolean
  require_consent: boolean
}

export interface PolicyTemplate {
  id: string
  project_id: string
  name: string
  description: string
  is_system: boolean
  base_credential_type?: string
  parameters: PolicyParameters
  current_version: number
  created_by?: string
  created_at: string
  updated_at: string
  // mapping #5: policy ↔ secret relation (2026-11-21)
  bound_count: number
  bound_secrets?: string
}

export interface PolicyTemplateVersion {
  id: string
  template_id: string
  version: number
  parameters: PolicyParameters
  change_note: string
  created_by?: string
  created_at: string
}

export interface PolicyTemplateRef {
  id: string
  name: string
  is_system: boolean
}

export interface SecretPolicyBinding {
  secret_id: string
  template?: PolicyTemplateRef
  template_version?: number
  override_parameters: Partial<PolicyParameters>
  override_warnings: string[]
  effective_policy: PolicyParameters
  applied_policy_source: string
}
