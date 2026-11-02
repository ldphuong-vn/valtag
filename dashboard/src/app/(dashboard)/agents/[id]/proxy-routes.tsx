'use client'

import { useEffect, useState } from 'react'
import { api } from '@/lib/api-client'
import type { ProxyRoute, Secret } from '@/types/api'
import { Copy, Plus, Trash2, Globe, Shield } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ProxyRoutesProps {
  agentId: string
}

export function ProxyRoutes({ agentId }: ProxyRoutesProps) {
  const [routes, setRoutes] = useState<ProxyRoute[]>([])
  const [secrets, setSecrets] = useState<Secret[]>([])
  const [adding, setAdding] = useState(false)
  const [copied, setCopied] = useState<string | null>(null)
  const [form, setForm] = useState({
    host_pattern: '',
    path_pattern: '/*',
    secret_id: '',
    injection_type: 'bearer',
    injection_key: 'Authorization',
    injection_format: 'Bearer {value}',
  })

  useEffect(() => {
    loadRoutes()
    api.secrets.list().then(r => setSecrets(r.secrets ?? [])).catch(() => null)
  }, [agentId])

  function loadRoutes() {
    api.proxyRoutes.list(agentId).then(setRoutes).catch(() => setRoutes([]))
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    await api.proxyRoutes.create({ agent_id: agentId, ...form })
    setAdding(false)
    setForm({ host_pattern: '', path_pattern: '/*', secret_id: '', injection_type: 'bearer', injection_key: 'Authorization', injection_format: 'Bearer {value}' })
    loadRoutes()
  }

  async function handleDelete(id: string) {
    await api.proxyRoutes.delete(id)
    loadRoutes()
  }

  function copyText(text: string, id: string) {
    navigator.clipboard.writeText(text)
    setCopied(id)
    setTimeout(() => setCopied(null), 2000)
  }

  function secretName(secretId: string) {
    return secrets.find(s => s.id === secretId)?.name ?? secretId.slice(0, 8)
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="font-semibold flex items-center gap-2">
          <Globe className="w-4 h-4" />Proxy Routes
        </h2>
        {!adding && (
          <button type="button" onClick={() => setAdding(true)} className="flex items-center gap-1 px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded hover:bg-primary/90">
            <Plus className="w-3 h-3" />Add Route
          </button>
        )}
      </div>

      {adding && (
        <form onSubmit={handleCreate} className="border rounded-lg p-4 space-y-3 bg-card">
          <h3 className="text-sm font-medium">Add Proxy Route</h3>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <label className="text-xs font-medium text-muted-foreground">Host Pattern</label>
              <input value={form.host_pattern} onChange={e => setForm(f => ({ ...f, host_pattern: e.target.value }))} placeholder="api.openai.com" className="w-full border rounded px-3 py-1.5 text-sm bg-background font-mono" required />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-medium text-muted-foreground">Path Pattern</label>
              <input value={form.path_pattern} onChange={e => setForm(f => ({ ...f, path_pattern: e.target.value }))} placeholder="/*" className="w-full border rounded px-3 py-1.5 text-sm bg-background font-mono" />
            </div>
          </div>
          <div className="space-y-1">
            <label className="text-xs font-medium text-muted-foreground">Secret</label>
            <select value={form.secret_id} onChange={e => setForm(f => ({ ...f, secret_id: e.target.value }))} className="w-full border rounded px-3 py-1.5 text-sm bg-background" required>
              <option value="">Select a secret…</option>
              {secrets.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>
          </div>
          <div className="grid grid-cols-3 gap-3">
            <div className="space-y-1">
              <label className="text-xs font-medium text-muted-foreground">Injection Type</label>
              <select value={form.injection_type} onChange={e => setForm(f => ({ ...f, injection_type: e.target.value }))} className="w-full border rounded px-3 py-1.5 text-sm bg-background">
                <option value="bearer">Bearer</option>
                <option value="header">Header</option>
                <option value="query">Query Param</option>
              </select>
            </div>
            <div className="space-y-1">
              <label className="text-xs font-medium text-muted-foreground">Header/Param Name</label>
              <input value={form.injection_key} onChange={e => setForm(f => ({ ...f, injection_key: e.target.value }))} className="w-full border rounded px-3 py-1.5 text-sm bg-background font-mono" />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-medium text-muted-foreground">Format</label>
              <input value={form.injection_format} onChange={e => setForm(f => ({ ...f, injection_format: e.target.value }))} placeholder="Bearer {value}" className="w-full border rounded px-3 py-1.5 text-sm bg-background font-mono" />
            </div>
          </div>
          <div className="flex gap-2 justify-end">
            <button type="button" onClick={() => setAdding(false)} className="px-3 py-1.5 text-sm border rounded hover:bg-muted">Cancel</button>
            <button type="submit" className="px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded hover:bg-primary/90">Create Route</button>
          </div>
        </form>
      )}

      {routes.length === 0 && !adding ? (
        <div className="text-center py-8 border rounded-lg">
          <Globe className="w-8 h-8 mx-auto text-muted-foreground mb-2" />
          <p className="text-sm text-muted-foreground">No proxy routes configured.</p>
          <p className="text-xs text-muted-foreground mt-1">Add a route to enable credential injection for this agent.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {routes.map(route => (
            <div key={route.id} className="border rounded-lg p-4 bg-card space-y-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <code className="text-sm font-mono font-semibold">{route.host_pattern}</code>
                  <code className="text-xs text-muted-foreground font-mono">{route.path_pattern}</code>
                  <span className={cn('px-2 py-0.5 rounded-full text-xs', route.enabled ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300' : 'bg-muted text-muted-foreground')}>
                    {route.enabled ? 'enabled' : 'disabled'}
                  </span>
                </div>
                <button type="button" onClick={() => handleDelete(route.id)} className="text-destructive hover:opacity-70" aria-label="Delete route">
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
              <dl className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
                <div><dt className="text-muted-foreground">Injection</dt><dd className="font-mono">{route.injection_type} → {route.injection_key}</dd></div>
                <div><dt className="text-muted-foreground">Secret</dt><dd className="font-mono">{secretName(route.secret_id)}</dd></div>
              </dl>
              {route.placeholder_key && (
                <div className="flex items-center gap-2 mt-1">
                  <Shield className="w-3 h-3 text-muted-foreground" />
                  <code className="text-xs font-mono bg-muted px-2 py-0.5 rounded">{route.placeholder_key}</code>
                  <button type="button" onClick={() => copyText(route.placeholder_key, route.id)} className="text-xs text-muted-foreground hover:text-foreground">
                    {copied === route.id ? 'Copied!' : <Copy className="w-3 h-3" />}
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {routes.length > 0 && (
        <div className="border rounded-lg p-4 bg-blue-50 dark:bg-blue-950 space-y-2">
          <h3 className="text-sm font-medium text-blue-700 dark:text-blue-300">Quick Setup</h3>
          <p className="text-xs text-blue-600 dark:text-blue-400">Configure your agent with these environment variables:</p>
          <div className="bg-white dark:bg-black rounded border p-3 space-y-1 font-mono text-xs">
            <div>HTTPS_PROXY=http://valtag-gateway:10256</div>
            {routes.map(r => (
              <div key={r.id}># {r.host_pattern}: {r.placeholder_key}</div>
            ))}
          </div>
          <button
            type="button"
            onClick={() => {
              const text = `HTTPS_PROXY=http://valtag-gateway:10256\n${routes.map(r => `# ${r.host_pattern}: ${r.placeholder_key}`).join('\n')}`
              navigator.clipboard.writeText(text)
            }}
            className="text-xs text-blue-700 dark:text-blue-300 hover:underline"
          >
            Copy All
          </button>
        </div>
      )}
    </div>
  )
}
