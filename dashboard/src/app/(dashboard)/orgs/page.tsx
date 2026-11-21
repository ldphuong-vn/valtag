'use client'

import { useEffect, useState } from 'react'
import { api } from '@/lib/api-client'
import type { Organization } from '@/types/api'
import { Building2, Plus, ArrowRight } from 'lucide-react'
import Link from 'next/link'

export default function OrgsPage() {
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState({ name: '', slug: '' })
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function loadOrgs() {
    setLoading(true)
    api.orgs.list()
      .then(r => setOrgs(r.orgs ?? []))
      .catch(e => setError((e as Error).message))
      .finally(() => setLoading(false))
  }

  useEffect(() => { loadOrgs() }, [])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      await api.orgs.create(form)
      setCreating(false)
      setForm({ name: '', slug: '' })
      loadOrgs()
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  function handleNameChange(e: React.ChangeEvent<HTMLInputElement>) {
    const name = e.target.value
    const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
    setForm({ name, slug })
  }

  return (
    <div className="p-6 max-w-3xl">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <Building2 className="w-5 h-5 text-muted-foreground" />
          <h1 className="text-xl font-semibold">Organizations</h1>
        </div>
        {!creating && !loading && orgs.length > 0 && (
          <span className="text-xs text-muted-foreground max-w-[280px] text-right">
            Each user belongs to a single organization on this server.
          </span>
        )}
        {!creating && !loading && orgs.length === 0 && (
          <button
            onClick={() => setCreating(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
          >
            <Plus className="w-4 h-4" />
            New Organization
          </button>
        )}
      </div>

      {error && (
        <div className="mb-6 p-4 border border-destructive/50 bg-destructive/10 rounded-lg text-sm text-destructive">
          {error}
        </div>
      )}

      {creating && (
        <form onSubmit={handleCreate} className="mb-6 p-4 border rounded-lg bg-card space-y-4">
          <h2 className="text-sm font-medium">Create Organization</h2>
          <div>
            <label htmlFor="org-name" className="block text-sm text-muted-foreground mb-1">Name</label>
            <input
              id="org-name"
              type="text"
              required
              value={form.name}
              onChange={handleNameChange}
              placeholder="My Organization"
              className="w-full px-3 py-2 text-sm border rounded-md bg-background focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
          <div>
            <label htmlFor="org-slug" className="block text-sm text-muted-foreground mb-1">Slug</label>
            <input
              id="org-slug"
              type="text"
              required
              value={form.slug}
              onChange={e => setForm(f => ({ ...f, slug: e.target.value }))}
              placeholder="my-organization"
              className="w-full px-3 py-2 text-sm border rounded-md bg-background focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
          <div className="flex gap-2 pt-2">
            <button
              type="submit"
              disabled={submitting}
              className="px-4 py-2 min-w-[100px] justify-center flex items-center rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
            >
              {submitting ? 'Creating…' : 'Create'}
            </button>
            <button
              type="button"
              onClick={() => { setCreating(false); setForm({ name: '', slug: '' }); setError(null) }}
              className="px-4 py-2 rounded-md text-sm text-muted-foreground hover:bg-accent transition-colors"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {loading ? (
        <div className="flex items-center justify-center p-12 border rounded-lg bg-card/50 border-dashed">
          <p className="text-sm text-muted-foreground animate-pulse">Loading organizations…</p>
        </div>
      ) : orgs.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-12 border rounded-lg bg-card/50 border-dashed text-center">
          <Building2 className="w-12 h-12 text-muted-foreground/50 mb-4" />
          <h3 className="text-lg font-medium mb-1">No organizations</h3>
          <p className="text-sm text-muted-foreground mb-4 max-w-sm">
            You aren't a member of any organizations yet. Create one to start managing workspaces and projects.
          </p>
          {!creating && (
            <button
              onClick={() => setCreating(true)}
              className="flex items-center gap-1.5 px-4 py-2 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
            >
              <Plus className="w-4 h-4" />
              Create Organization
            </button>
          )}
        </div>
      ) : (
        <ul className="space-y-3">
          {orgs.map(org => (
            <li key={org.id} className="p-4 border rounded-lg bg-card hover:border-primary/50 transition-colors group">
              <div className="flex items-center gap-4">
                <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
                  <Building2 className="w-5 h-5 text-primary" />
                </div>
                <div className="min-w-0 flex-1">
                  <h3 className="text-base font-medium truncate">{org.name}</h3>
                  <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                    <p className="text-xs text-muted-foreground font-mono bg-muted px-1.5 py-0.5 rounded">{org.slug}</p>
                    <span className="text-xs text-muted-foreground capitalize border px-1.5 py-0.5 rounded-full">{org.plan}</span>
                    {/* org → workspace → project counts (mapping #7) */}
                    {typeof org.workspace_count === 'number' && (
                      <span className="text-xs text-muted-foreground border px-1.5 py-0.5 rounded-full">
                        {org.workspace_count} workspace{org.workspace_count === 1 ? '' : 's'}
                      </span>
                    )}
                    {typeof org.project_count === 'number' && (
                      <span className="text-xs text-muted-foreground border px-1.5 py-0.5 rounded-full">
                        {org.project_count} project{org.project_count === 1 ? '' : 's'}
                      </span>
                    )}
                  </div>
                </div>
                <Link
                  href={`/orgs/${org.id}`}
                  className="flex items-center gap-2 px-4 py-2 min-h-[44px] text-sm font-medium text-primary bg-primary/10 rounded-md hover:bg-primary/20 focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2"
                >
                  View Workspaces
                  <ArrowRight className="w-4 h-4" />
                </Link>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
