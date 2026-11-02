'use client'

import { useEffect, useState } from 'react'
import { api } from '@/lib/api-client'
import type { Workspace } from '@/types/api'
import { Layers, CheckCircle2, ArrowLeft } from 'lucide-react'
import Link from 'next/link'
import { useParams } from 'next/navigation'

export default function WorkspaceListPage() {
  const params = useParams<{ orgId: string }>()
  const orgId = params.orgId

  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [currentWorkspaceId, setCurrentWorkspaceId] = useState<string | null>(null)

  function loadWorkspaces() {
    setLoading(true)
    setError(null)
    api.workspaces.list(orgId)
      .then(r => setWorkspaces(r.workspaces ?? []))
      .catch(e => setError((e as Error).message))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    loadWorkspaces()
    const wid = typeof window !== 'undefined' ? localStorage.getItem('valt_current_workspace') : null
    setCurrentWorkspaceId(wid)
  }, [orgId])

  function selectWorkspace(workspaceId: string, workspaceName: string) {
    if (typeof window !== 'undefined') {
      localStorage.setItem('valt_current_org', orgId)
      localStorage.setItem('valt_current_workspace', workspaceId)
      localStorage.setItem('valt_current_workspace_name', workspaceName)
      localStorage.removeItem('valt_current_project')
      window.dispatchEvent(new Event('valtag:workspace-changed'))
      window.dispatchEvent(new Event('valtag:project-changed'))
      setCurrentWorkspaceId(workspaceId)
    }
  }

  return (
    <div className="p-6 max-w-3xl">
      <Link 
        href="/orgs" 
        className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-6 min-h-[44px] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 rounded-md px-2 -ml-2"
      >
        <ArrowLeft className="w-4 h-4" />
        Back to Organizations
      </Link>

      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <Layers className="w-5 h-5 text-muted-foreground" />
          <h1 className="text-xl font-semibold">Workspaces</h1>
        </div>
        <Link href="/projects" className="text-sm text-primary hover:underline">
          Go to Projects
        </Link>
      </div>

      {error && (
        <div className="mb-6 p-4 border border-destructive/50 bg-destructive/10 rounded-lg text-sm text-destructive">
          {error}
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center p-12 border rounded-lg bg-card/50 border-dashed">
          <p className="text-sm text-muted-foreground animate-pulse">Loading workspaces…</p>
        </div>
      ) : workspaces.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-12 border rounded-lg bg-card/50 border-dashed text-center">
          <Layers className="w-12 h-12 text-muted-foreground/50 mb-4" />
          <h3 className="text-lg font-medium mb-1">No workspaces</h3>
          <p className="text-sm text-muted-foreground mb-6 max-w-sm">
            This organization doesn't have any workspaces yet. Ask your org admin to create one.
          </p>
          <Link
            href="/orgs"
            className="inline-flex items-center justify-center rounded-md border px-4 py-2 text-sm hover:bg-accent"
          >
            Back to Organizations
          </Link>
        </div>
      ) : (
        <ul className="space-y-4">
          {workspaces.map(ws => (
            <li key={ws.id} className="p-5 border rounded-lg bg-card hover:border-primary/30 transition-colors shadow-sm">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                <div className="flex items-center gap-4">
                  <div className="w-12 h-12 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
                    <Layers className="w-6 h-6 text-primary" />
                  </div>
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <h3 className="text-base font-semibold truncate">{ws.name}</h3>
                      {currentWorkspaceId === ws.id && (
                        <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-700 bg-emerald-100 px-2 py-0.5 rounded-full border border-emerald-200">
                          <CheckCircle2 className="w-3.5 h-3.5" /> Current
                        </span>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground font-mono mt-1">{ws.slug}</p>
                  </div>
                </div>
                
                <button
                  onClick={() => selectWorkspace(ws.id, ws.name)}
                  disabled={currentWorkspaceId === ws.id}
                  className="px-4 py-2 min-h-[44px] min-w-[140px] flex items-center justify-center rounded-md border text-sm font-medium hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors w-full sm:w-auto"
                >
                  {currentWorkspaceId === ws.id ? 'Current Workspace' : 'Set current'}
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
