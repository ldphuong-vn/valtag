'use client'

import { useState, useEffect, useCallback } from 'react'
import Link from 'next/link'
import { api } from '@/lib/api-client'
import type { DynamicProvider } from '@/types/api'
import { CreateProviderForm } from './create-provider-form'

// Providers live inside a project; follow the app-wide "current project"
// (localStorage `valt_current_project`, set on the Projects page).
const readProjectId = () =>
  typeof window === 'undefined' ? '' : localStorage.getItem('valt_current_project') ?? ''

export default function ProvidersPage() {
  const [providers, setProviders] = useState<DynamicProvider[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [projectId, setProjectId] = useState(readProjectId)

  useEffect(() => {
    const sync = () => setProjectId(readProjectId())
    sync()
    window.addEventListener('valtag:project-changed', sync)
    window.addEventListener('storage', sync)
    return () => {
      window.removeEventListener('valtag:project-changed', sync)
      window.removeEventListener('storage', sync)
    }
  }, [])

  const fetchProviders = useCallback(async () => {
    if (!projectId) {
      setLoading(false)
      return
    }
    try {
      const res = await api.providers.list(projectId)
      setProviders(res.providers ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load providers')
    } finally {
      setLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    fetchProviders()
  }, [fetchProviders])

  const handleCreated = async () => {
    setShowForm(false)
    await fetchProviders()
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Dynamic Providers</h1>
          <p className="text-sm text-muted-foreground mt-1">
            On-demand credential generation from configured backends.
          </p>
        </div>
        <button
          onClick={() => setShowForm(v => !v)}
          className="px-4 py-2 bg-primary text-primary-foreground text-sm rounded-md hover:bg-primary/90 transition-colors"
        >
          {showForm ? 'Cancel' : 'Add Provider'}
        </button>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {showForm && !projectId && (
        <div className="rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          No project selected. <Link href="/projects" className="underline">Pick a project</Link> first — providers are created inside a project.
        </div>
      )}

      {showForm && projectId && (
        <CreateProviderForm
          projectId={projectId}
          onCreated={handleCreated}
          onCancel={() => setShowForm(false)}
        />
      )}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading providers…</p>
      ) : !projectId ? (
        <div className="rounded-lg border bg-card p-10 text-center text-sm text-muted-foreground">
          No project selected. <Link href="/projects" className="underline text-primary hover:underline">Pick a project</Link> to see its providers.
        </div>
      ) : providers.length === 0 ? (
        <div className="rounded-lg border bg-card p-10 text-center text-sm text-muted-foreground">
          No providers configured. Add one to start generating dynamic credentials.
        </div>
      ) : (
        <div className="rounded-lg border bg-card overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40">
                <th className="px-4 py-3 text-left font-medium">Name</th>
                <th className="px-4 py-3 text-left font-medium">Type</th>
                <th className="px-4 py-3 text-left font-medium">Status</th>
                <th className="px-4 py-3 text-left font-medium">Created</th>
                <th className="px-4 py-3 text-left font-medium">Leases</th>
              </tr>
            </thead>
            <tbody>
              {providers.map(p => (
                <tr key={p.id} className="border-b last:border-0 hover:bg-muted/20 transition-colors">
                  <td className="px-4 py-3 font-medium">{p.name}</td>
                  <td className="px-4 py-3">
                    <span className="inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium">
                      {p.provider_type}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                      p.status === 'active'
                        ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                        : 'bg-muted text-muted-foreground'
                    }`}>
                      {p.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground font-mono text-xs">
                    {new Date(p.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3">
                    <a
                      href={`/providers/${p.id}/leases`}
                      className="text-primary hover:underline text-xs"
                    >
                      View Leases
                    </a>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
