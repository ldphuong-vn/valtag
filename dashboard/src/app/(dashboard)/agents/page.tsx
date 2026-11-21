'use client'

import { useEffect, useState } from 'react'
import { api } from '@/lib/api-client'
import type { AgentIdentity } from '@/types/api'
import { Bot, Plus, AlertCircle } from 'lucide-react'
import { cn } from '@/lib/utils'

const AGENT_TYPES = ['claude', 'cursor', 'opencode', 'custom']
const SCOPE_OPTIONS = ['secrets:read', 'secrets:write', 'approvals:request']

type CreateForm = { name: string; description: string; agent_type: string; max_session_ttl: number; allowed_scopes: string[] }
const DEFAULT_FORM: CreateForm = { name: '', description: '', agent_type: 'custom', max_session_ttl: 3600, allowed_scopes: [] }

export default function AgentsPage() {
  const [projectId, setProjectId] = useState<string | null>(null)
  const [agents, setAgents] = useState<AgentIdentity[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [form, setForm] = useState<CreateForm>(DEFAULT_FORM)

  useEffect(() => {
    // reload on context change — switching org/workspace from the sidebar must
    // refresh this list (it reads valt_current_project)
    const load = () => {
      const pid = typeof window !== 'undefined' ? localStorage.getItem('valt_current_project') : null
      setProjectId(pid)
      if (pid) {
        api.agents.list(pid)
          .then(r => setAgents(r.agents ?? []))
          .catch(() => setAgents([]))
          .finally(() => setLoading(false))
      } else {
        setAgents([])
        setLoading(false)
      }
    }
    load()
    window.addEventListener('valtag:project-changed', load)
    window.addEventListener('valtag:workspace-changed', load)
    window.addEventListener('storage', load)
    return () => {
      window.removeEventListener('valtag:project-changed', load)
      window.removeEventListener('valtag:workspace-changed', load)
      window.removeEventListener('storage', load)
    }
  }, [])

  function toggleScope(scope: string) {
    setForm(f => ({
      ...f,
      allowed_scopes: f.allowed_scopes.includes(scope) ? f.allowed_scopes.filter(s => s !== scope) : [...f.allowed_scopes, scope],
    }))
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!projectId) return
    setSubmitting(true)
    try {
      await api.agents.create(projectId, form)
      setCreating(false)
      setForm(DEFAULT_FORM)
      api.agents.list(projectId).then(r => setAgents(r.agents ?? []))
    } finally {
      setSubmitting(false)
    }
  }

  if (!projectId) {
    return (
      <div className="flex flex-col items-center justify-center h-64 gap-3 text-muted-foreground">
        <AlertCircle className="w-8 h-8" />
        <p>No project selected. <a href="/projects" className="text-primary underline">Select a project</a> first.</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Agents</h1>
          <p className="text-sm text-muted-foreground mt-1">Manage AI agent identities and their access.</p>
        </div>
        <button onClick={() => setCreating(true)} className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm font-medium hover:bg-primary/90">
          <Plus className="w-4 h-4" />Register Agent
        </button>
      </div>

      {creating && (
        <form onSubmit={handleCreate} className="border rounded-lg p-4 space-y-4 bg-card">
          <h2 className="font-medium">Register New Agent</h2>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1">
              <label className="text-sm font-medium">Name *</label>
              <input required value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} className="w-full border rounded px-3 py-1.5 text-sm bg-background" placeholder="my-agent" />
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium">Agent Type</label>
              <select value={form.agent_type} onChange={e => setForm(f => ({ ...f, agent_type: e.target.value }))} className="w-full border rounded px-3 py-1.5 text-sm bg-background">
                {AGENT_TYPES.map(t => <option key={t} value={t}>{t}</option>)}
              </select>
            </div>
            <div className="space-y-1 col-span-2">
              <label className="text-sm font-medium">Description</label>
              <input value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))} className="w-full border rounded px-3 py-1.5 text-sm bg-background" placeholder="Optional" />
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium">Max Session TTL (seconds)</label>
              <input type="number" min={60} value={form.max_session_ttl} onChange={e => setForm(f => ({ ...f, max_session_ttl: parseInt(e.target.value) || 3600 }))} className="w-full border rounded px-3 py-1.5 text-sm bg-background" />
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium">Allowed Scopes</label>
              <div className="flex flex-wrap gap-2 pt-1">
                {SCOPE_OPTIONS.map(s => (
                  <button key={s} type="button" onClick={() => toggleScope(s)} className={cn('px-2 py-0.5 rounded text-xs border', form.allowed_scopes.includes(s) ? 'bg-primary text-primary-foreground border-primary' : 'bg-background border-border')}>
                    {s}
                  </button>
                ))}
              </div>
            </div>
          </div>
          <div className="flex gap-2 justify-end">
            <button type="button" onClick={() => setCreating(false)} className="px-3 py-1.5 text-sm border rounded hover:bg-muted">Cancel</button>
            <button type="submit" disabled={submitting} className="px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded hover:bg-primary/90 disabled:opacity-50">
              {submitting ? 'Registering…' : 'Register'}
            </button>
          </div>
        </form>
      )}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading agents…</p>
      ) : agents.length === 0 ? (
        <div className="flex flex-col items-center justify-center h-40 gap-2 text-muted-foreground border rounded-lg">
          <Bot className="w-8 h-8" /><p className="text-sm">No agents registered yet.</p>
        </div>
      ) : (
        <div className="border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50 border-b">
              <tr>
                <th className="text-left px-4 py-2 font-medium">Name</th>
                <th className="text-left px-4 py-2 font-medium">Type</th>
                <th className="text-left px-4 py-2 font-medium">Status</th>
                <th className="text-left px-4 py-2 font-medium">Created</th>
                <th className="px-4 py-2" />
              </tr>
            </thead>
            <tbody className="divide-y">
              {agents.map(agent => (
                <tr key={agent.id} className="hover:bg-muted/30">
                  <td className="px-4 py-2 font-medium">
                    <span className="flex items-center gap-2"><Bot className="w-4 h-4 text-muted-foreground" />{agent.name}</span>
                  </td>
                  <td className="px-4 py-2"><span className="px-2 py-0.5 rounded-full text-xs bg-secondary text-secondary-foreground">{agent.agent_type}</span></td>
                  <td className="px-4 py-2">
                    <span className={cn('px-2 py-0.5 rounded-full text-xs', agent.status === 'active' ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300' : 'bg-muted text-muted-foreground')}>
                      {agent.status}
                    </span>
                  </td>
                  <td className="px-4 py-2 text-muted-foreground">{new Date(agent.created_at).toLocaleDateString()}</td>
                  <td className="px-4 py-2 text-right"><a href={`/agents/${agent.id}`} className="text-primary text-xs hover:underline">Manage</a></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
