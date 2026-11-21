import type {
  Secret,
  AccessRequest,
  Credential,
  AuditLog,
  Organization,
  Workspace,
  Project,
  OrgMembership,
  ProjectMembership,
  AgentIdentity,
  AgentToken,
  ScanResult,
  ScanFinding,
  DynamicProvider,
  DynamicLease,
  NotificationChannel,
  PolicyParameters,
  PolicyTemplate,
  PolicyTemplateVersion,
  SecretPolicyBinding,
  ProxyRoute,
  EndpointLimit,
} from '@/types/api'

const BASE = '/api/proxy'
export class ApiError extends Error {
  constructor(message: string, public status: number) { super(message); this.name = 'ApiError' }
}

// Attempt a silent token refresh via the server-side refresh endpoint.
// Returns true if refresh succeeded (new cookies set), false otherwise.
async function tryRefresh(): Promise<boolean> {
  try {
    const res = await fetch('/api/auth/refresh', { method: 'POST' })
    return res.ok
  } catch {
    return false
  }
}

// Build a query string, silently dropping empty params — URLSearchParams
// would otherwise serialize undefined as the literal string "undefined"
// (bug 2026-11-21: admin.users({search: undefined}) filtered
// email ILIKE '%undefined%' and returned zero users).
function toQuery(params?: Record<string, string | number | undefined>): string {
  if (!params) return ''
  const q = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') q.set(key, String(value))
  }
  return q.toString()
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...options?.headers },
  })
  if (res.status === 401) {
    // Try silent refresh once before redirecting to login.
    const refreshed = await tryRefresh()
    if (refreshed) {
      const retryRes = await fetch(`${BASE}${path}`, {
        ...options,
        headers: { 'Content-Type': 'application/json', ...options?.headers },
      })
      if (retryRes.status === 401) {
        window.location.href = '/login'
        throw new Error('Unauthorized')
      }
      if (!retryRes.ok) {
        const err = await retryRes.json().catch(() => ({ error: retryRes.statusText }))
        throw new ApiError((err as { error?: string }).error ?? 'Request failed', retryRes.status)
      }
      return retryRes.json() as Promise<T>
    }
    window.location.href = '/login'
    throw new Error('Unauthorized')
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new ApiError((err as { error?: string }).error ?? 'Request failed', res.status)
  }
  return res.json() as Promise<T>
}

export const api = {
  secrets: {
    list: () => apiFetch<{ secrets: Secret[] }>('/secrets'),
    get: (id: string) => apiFetch<Secret>(`/secrets/${id}`),
    create: (body: Partial<Secret> & { value: string; project_id?: string }) =>
      apiFetch<Secret>('/secrets', { method: 'POST', body: JSON.stringify(body) }),
    update: (id: string, body: Partial<Secret>) =>
      apiFetch<Secret>(`/secrets/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
    delete: (id: string) => apiFetch<void>(`/secrets/${id}`, { method: 'DELETE' }),
  },
  requests: {
    list: (status?: string) =>
      apiFetch<{ requests: AccessRequest[] }>(`/access-requests${status ? `?status=${status}` : ''}`),
    create: (secretId: string, body: { reason: string; duration_minutes: number }) =>
      apiFetch<AccessRequest>(`/secrets/${secretId}/access-requests`, {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    approve: (id: string, body: { reason?: string }) =>
      apiFetch<void>(`/access-requests/${id}/approve`, { method: 'POST', body: JSON.stringify(body) }),
    reject: (id: string, body: { reason: string }) =>
      apiFetch<void>(`/access-requests/${id}/reject`, { method: 'POST', body: JSON.stringify(body) }),
  },
  credentials: {
    get: (requestId: string) => apiFetch<Credential>(`/credentials/${requestId}`),
    revoke: (requestId: string) =>
      apiFetch<void>(`/credentials/${requestId}/revoke`, { method: 'POST' }),
  },
  audit: {
    list: (params?: { page?: number; limit?: number }) => {
      const q = toQuery(params)
      return apiFetch<{ logs: AuditLog[] }>(`/audit/logs${q ? `?${q}` : ''}`)
    },
  },
  orgs: {
    list: async () => {
      const data = await apiFetch<{ orgs: Organization[] } | Organization[]>('/orgs')
      return Array.isArray(data) ? { orgs: data } : { orgs: data.orgs ?? [] }
    },
    create: (body: { name: string; slug: string }) =>
      apiFetch<Organization>('/orgs', { method: 'POST', body: JSON.stringify(body) }),
    getMembers: (orgId: string) =>
      apiFetch<{ members: OrgMembership[] }>(`/orgs/${orgId}/members`),
    addMember: (orgId: string, body: { user_id: string; role: string }) =>
      apiFetch<OrgMembership>(`/orgs/${orgId}/members`, { method: 'POST', body: JSON.stringify(body) }),
  },
  workspaces: {
    list: async (orgId: string) => {
      const data = await apiFetch<{ workspaces: Workspace[] } | Workspace[]>(`/orgs/${orgId}/workspaces`)
      return Array.isArray(data) ? { workspaces: data } : { workspaces: data.workspaces ?? [] }
    },
    create: (orgId: string, body: { name: string; slug: string }) =>
      apiFetch<Workspace>(`/orgs/${orgId}/workspaces`, { method: 'POST', body: JSON.stringify(body) }),
  },
  projects: {
    list: async (workspaceId: string) => {
      const data = await apiFetch<{ projects: Project[] } | Project[]>(`/workspaces/${workspaceId}/projects`)
      return Array.isArray(data) ? { projects: data } : { projects: data.projects ?? [] }
    },
    get: (projectId: string) => apiFetch<Project>(`/projects/${projectId}`),
    create: (workspaceId: string, body: { name: string; slug: string }) =>
      apiFetch<Project>(`/workspaces/${workspaceId}/projects`, { method: 'POST', body: JSON.stringify(body) }),
    getMembers: (projectId: string) =>
      apiFetch<{ members: ProjectMembership[] }>(`/projects/${projectId}/members`),
    addMember: (projectId: string, body: { user_id: string; role: string }) =>
      apiFetch<ProjectMembership>(`/projects/${projectId}/members`, { method: 'POST', body: JSON.stringify(body) }),
  },
  agents: {
    list: async (projectId: string) => {
      const data = await apiFetch<{ agents: AgentIdentity[] } | AgentIdentity[]>(`/projects/${projectId}/agents`)
      return Array.isArray(data) ? { agents: data } : { agents: data.agents ?? [] }
    },
    listLeases: (agentId: string) =>
      apiFetch<{
        leases: Array<{ id: string; provider_id: string; ttl_seconds: number; expires_at: string; created_at: string }>
      }>(`/agents/${agentId}/leases`),
    create: (projectId: string, body: {
      name: string
      description?: string
      agent_type: string
      max_session_ttl: number
      allowed_scopes: string[]
    }) => apiFetch<AgentIdentity>(`/projects/${projectId}/agents`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
    get: (agentId: string) =>
      apiFetch<AgentIdentity>(`/agents/${agentId}`),
    issueToken: (agentId: string, body: { scopes: string[]; expires_in_seconds?: number }) =>
      apiFetch<AgentToken>(`/agents/${agentId}/tokens`, {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    revokeToken: (agentId: string, tokenId: string) =>
      apiFetch<void>(`/agents/${agentId}/tokens/${tokenId}`, { method: 'DELETE' }),
    listTokens: async (agentId: string) => {
      const data = await apiFetch<{ tokens: AgentToken[] } | AgentToken[]>(`/agents/${agentId}/tokens`)
      return Array.isArray(data) ? { tokens: data } : { tokens: data.tokens ?? [] }
    },
  },
  providers: {
    list: (projectId: string) =>
      apiFetch<{ providers: DynamicProvider[] }>(`/projects/${projectId}/providers`),
    create: (projectId: string, body: { name: string; provider_type: string; config: Record<string, string> }) =>
      apiFetch<DynamicProvider>(`/projects/${projectId}/providers`, { method: 'POST', body: JSON.stringify(body) }),
    createLease: (providerId: string, body: { agent_id?: string; ttl_seconds: number }) =>
      apiFetch<DynamicLease>(`/providers/${providerId}/leases`, { method: 'POST', body: JSON.stringify(body) }),
    revokeLease: (leaseId: string) =>
      apiFetch<void>(`/leases/${leaseId}`, { method: 'DELETE' }),
    listLeases: (providerId: string) =>
      apiFetch<{ leases: DynamicLease[] }>(`/providers/${providerId}/leases`),
  },
  policy: {
    getSecret: (secretId: string) => apiFetch<Record<string, unknown>>(`/secrets/${secretId}/policy`),
    putSecret: (secretId: string, policy: Record<string, unknown>) =>
      apiFetch<Record<string, unknown>>(`/secrets/${secretId}/policy`, {
        method: 'PUT',
        body: JSON.stringify(policy),
      }),
    getProject: (projectId: string) => apiFetch<Record<string, unknown>>(`/projects/${projectId}/policy`),
    putProject: (projectId: string, policy: Record<string, unknown>) =>
      apiFetch<Record<string, unknown>>(`/projects/${projectId}/policy`, {
        method: 'PUT',
        body: JSON.stringify(policy),
      }),
  },
  notificationChannels: {
    list: () => apiFetch<{ channels: NotificationChannel[] }>('/me/notification-channels'),
    upsert: (channelType: string, handle: string) =>
      apiFetch<NotificationChannel>('/me/notification-channels', {
        method: 'POST',
        body: JSON.stringify({ channel_type: channelType, handle }),
      }),
    delete: (id: string) =>
      apiFetch<void>(`/me/notification-channels/${id}`, { method: 'DELETE' }),
    telegramLink: () =>
      apiFetch<{ url: string }>('/me/telegram-link', { method: 'POST' }),
  },
  integrations: {
    list: (orgId: string) =>
      apiFetch<{ integrations: Array<{ id: string; org_id: string; provider: string; workspace_id: string; team_name: string; created_at: string }> }>(`/integrations?org_id=${orgId}`),
    disconnectSlack: (orgId: string) =>
      apiFetch<void>(`/integrations/slack?org_id=${orgId}`, { method: 'DELETE' }),
  },
  scans: {
    list: (projectId: string) =>
      apiFetch<{ scans: ScanResult[] }>(`/projects/${projectId}/scans`),
    getFindings: (scanId: string) =>
      apiFetch<{ findings: ScanFinding[] }>(`/scans/${scanId}/findings`),
    importFinding: (scanId: string, findingId: string, secretId: string) =>
      apiFetch<void>(`/scans/${scanId}/findings/${findingId}/import`, {
        method: 'POST',
        body: JSON.stringify({ secret_id: secretId }),
      }),
    dismissFinding: (scanId: string, findingId: string) =>
      apiFetch<void>(`/scans/${scanId}/findings/${findingId}/dismiss`, { method: 'POST' }),
  },
  proxyRoutes: {
    list: (agentId: string) =>
      apiFetch<ProxyRoute[]>(`/proxy-routes?agent_id=${agentId}`),
    create: (body: {
      agent_id: string
      host_pattern: string
      path_pattern?: string
      secret_id: string
      injection_type?: string
      injection_key?: string
      injection_format?: string
    }) => apiFetch<ProxyRoute>('/proxy-routes', { method: 'POST', body: JSON.stringify(body) }),
    update: (id: string, body: Partial<ProxyRoute>) =>
      apiFetch<{ status: string }>(`/proxy-routes/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
    delete: (id: string) =>
      apiFetch<void>(`/proxy-routes/${id}`, { method: 'DELETE' }),
  },
  admin: {
    stats: () => apiFetch<{
      total_users: number
      total_orgs: number
      total_secrets: number
      total_agents: number
      total_requests_today: number
      total_requests_all: number
      users_today: number
    }>('/admin/stats'),
    leases: () =>
      apiFetch<{
        leases: Array<{
          id: string
          provider_id: string
          provider_name: string
          agent_id?: string
          ttl_seconds: number
          expires_at: string
          created_at: string
        }>
      }>('/admin/leases'),
    users: (params?: { page?: number; limit?: number; search?: string }) => {
      const q = toQuery(params)
      return apiFetch<{
        users: Array<{
          id: string
          email: string
          status: string
          email_verified: boolean
          totp_enabled: boolean
          role: string
          created_at: string
          orgs?: string
        }>
        page: number
        limit: number
      }>(`/admin/users${q ? `?${q}` : ''}`)
    },
    orgs: (params?: { page?: number; limit?: number }) => {
      const q = toQuery(params)
      return apiFetch<{
        orgs: Array<{
          id: string
          name: string
          slug: string
          owner_id: string
          member_count: number
          secrets_count: number
          created_at: string
        }>
        page: number
        limit: number
      }>(`/admin/orgs${q ? `?${q}` : ''}`)
    },
  },
  endpointLimits: {
    list: (agentId: string) =>
      apiFetch<EndpointLimit[]>(`/proxy-endpoint-limits?agent_id=${agentId}`),
    create: (body: {
      agent_id: string
      host_pattern: string
      path_pattern?: string
      rpm?: number
      blocked?: boolean
    }) => apiFetch<EndpointLimit>('/proxy-endpoint-limits', { method: 'POST', body: JSON.stringify(body) }),
    delete: (id: string) =>
      apiFetch<void>(`/proxy-endpoint-limits/${id}`, { method: 'DELETE' }),
  },
  policies: {
    listTemplates: (projectId: string) =>
      apiFetch<{ templates: PolicyTemplate[] }>(`/projects/${projectId}/policy-templates`),
    createTemplate: (
      projectId: string,
      body: {
        name: string
        description: string
        base_credential_type?: string
        parameters: PolicyParameters
      }
    ) => apiFetch<PolicyTemplate>(`/projects/${projectId}/policy-templates`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
    getTemplate: (templateId: string) =>
      apiFetch<PolicyTemplate>(`/policy-templates/${templateId}`),
    updateTemplate: (
      templateId: string,
      body: { parameters: PolicyParameters; change_note: string }
    ) => apiFetch<PolicyTemplateVersion>(`/policy-templates/${templateId}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
    cloneTemplate: (templateId: string, name: string) =>
      apiFetch<PolicyTemplate>(`/policy-templates/${templateId}/clone`, {
        method: 'POST',
        body: JSON.stringify({ name }),
      }),
    listVersions: (templateId: string) =>
      apiFetch<{ versions: PolicyTemplateVersion[] }>(`/policy-templates/${templateId}/versions`),
    getBinding: (secretId: string) =>
      apiFetch<SecretPolicyBinding>(`/secrets/${secretId}/policy-binding`),
    updateBinding: (
      secretId: string,
      body: { template_id: string; template_version: number; override_parameters?: Record<string, unknown> }
    ) => apiFetch<SecretPolicyBinding>(`/secrets/${secretId}/policy-binding`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  },
}
