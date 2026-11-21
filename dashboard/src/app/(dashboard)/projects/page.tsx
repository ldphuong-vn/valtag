'use client'

import { useEffect, useState } from 'react'
import { ApiError, api } from '@/lib/api-client'
import type { Project } from '@/types/api'
import { FolderOpen, CheckCircle2, ArrowRight, Plus } from 'lucide-react'
import Link from 'next/link'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'

function slugify(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40)
}

export default function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [currentProjectId, setCurrentProjectId] = useState<string | null>(null)
  const [hasWorkspace, setHasWorkspace] = useState(false)
  const [workspaceId, setWorkspaceId] = useState('')

  // create-project form (server API existed — POST /workspaces/{id}/projects —
  // the dashboard just never had a button; anh, 2026-11-21)
  const [showCreate, setShowCreate] = useState(false)
  const [projName, setProjName] = useState('')
  const [projSlug, setProjSlug] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    const load = async () => {
      const workspaceId = localStorage.getItem('valt_current_workspace')
      setHasWorkspace(!!workspaceId)
      setWorkspaceId(workspaceId ?? '')
      setError(null)

      if (!workspaceId) {
        setProjects([])
        setCurrentProjectId(null)
        setLoading(false)
        return
      }

      setLoading(true)
      try {
        const r = await api.projects.list(workspaceId)
        setProjects(r.projects ?? [])
      } catch (e) {
        if (e instanceof ApiError && (e.status === 403 || e.status === 404)) {
          localStorage.removeItem('valt_current_org')
          localStorage.removeItem('valt_current_workspace')
          localStorage.removeItem('valt_current_workspace_name')
          localStorage.removeItem('valt_current_project')
          window.dispatchEvent(new Event('valtag:workspace-changed'))
          window.dispatchEvent(new Event('valtag:project-changed'))
          setHasWorkspace(false)
          setProjects([])
          setCurrentProjectId(null)
          setError('Selected workspace is no longer accessible. Please choose another workspace.')
          return
        }
        setError((e as Error).message)
      } finally {
        setLoading(false)
      }

      setCurrentProjectId(localStorage.getItem('valt_current_project'))
    }

    void load()
    window.addEventListener('storage', load)
    window.addEventListener('valtag:workspace-changed', load)
    return () => {
      window.removeEventListener('storage', load)
      window.removeEventListener('valtag:workspace-changed', load)
    }
  }, [])

  function selectProject(projectId: string) {
    localStorage.setItem('valt_current_project', projectId)
    window.dispatchEvent(new Event('valtag:project-changed'))
    setCurrentProjectId(projectId)
  }

  async function handleCreate() {
    if (!projName.trim() || !projSlug.trim() || !workspaceId) return
    setCreating(true)
    setError(null)
    try {
      const created = await api.projects.create(workspaceId, { name: projName.trim(), slug: projSlug.trim() })
      setShowCreate(false)
      setProjName(''); setProjSlug(''); setSlugTouched(false)
      if (created?.id) selectProject(created.id)
      window.dispatchEvent(new Event('valtag:project-changed'))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create project')
    } finally {
      setCreating(false)
    }
  }

  if (!hasWorkspace) {
    return (
      <div className="p-6 max-w-xl">
        <div className="flex items-center gap-3 mb-6">
          <FolderOpen className="w-5 h-5 text-muted-foreground" />
          <h1 className="text-xl font-semibold">Projects</h1>
        </div>
        <div className="flex flex-col items-center justify-center p-12 border rounded-lg bg-card/50 border-dashed text-center">
          <div className="w-12 h-12 rounded-full bg-primary/10 flex items-center justify-center mb-4">
            <FolderOpen className="w-6 h-6 text-primary" />
          </div>
          <h3 className="text-lg font-medium mb-2">No Workspace Selected</h3>
          <p className="text-sm text-muted-foreground mb-6 max-w-sm">
            Projects are organized within workspaces. To view or create projects, please select a workspace from your organization first.
          </p>
          <Link
            href="/overview"
            className="inline-flex items-center gap-2 px-4 py-2 min-h-[44px] justify-center rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            Pick a workspace in Overview
            <ArrowRight className="w-4 h-4" />
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className="p-6 max-w-3xl">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <FolderOpen className="w-5 h-5 text-muted-foreground" />
          <h1 className="text-xl font-semibold">Projects</h1>
        </div>
        <Button onClick={() => { setShowCreate(true); setProjName(''); setProjSlug(''); setSlugTouched(false); setError(null) }}>
          <Plus className="w-4 h-4" /> New Project
        </Button>
      </div>

      {error && (
        <div className="mb-6 p-4 border border-destructive/50 bg-destructive/10 rounded-lg text-sm text-destructive">
          {error}
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center p-12 border rounded-lg bg-card/50 border-dashed">
          <p className="text-sm text-muted-foreground animate-pulse">Loading projects…</p>
        </div>
      ) : projects.length === 0 ? (
        <div className="flex flex-col items-center justify-center p-12 border rounded-lg bg-card/50 border-dashed text-center">
          <FolderOpen className="w-12 h-12 text-muted-foreground/50 mb-4" />
          <h3 className="text-lg font-medium mb-1">No projects found</h3>
          <p className="text-sm text-muted-foreground mb-6">This workspace doesn&apos;t have any projects yet.</p>
          <Button onClick={() => { setShowCreate(true); setProjName(''); setProjSlug(''); setSlugTouched(false) }}>
            <Plus className="w-4 h-4" /> Create the first project
          </Button>
        </div>
      ) : (
        <ul className="space-y-3">
          {projects.map(project => (
            <li key={project.id} className="p-5 border rounded-lg bg-card hover:border-primary/50 transition-colors">
              <div className="flex items-start justify-between gap-4">
                <div className="flex items-center gap-4">
                  <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
                    <FolderOpen className="w-5 h-5 text-primary" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <h3 className="text-base font-medium truncate">{project.name}</h3>
                      {currentProjectId === project.id && (
                        <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-600 bg-emerald-50 px-2 py-0.5 rounded-full border border-emerald-200">
                          <CheckCircle2 className="w-3.5 h-3.5" /> Current
                        </span>
                      )}
                    </div>
                    <p className="text-xs text-muted-foreground font-mono mt-1">{project.slug}</p>
                  </div>
                </div>
              </div>
              <div className="mt-4 flex items-center gap-3">
                <button
                  onClick={() => selectProject(project.id)}
                  disabled={currentProjectId === project.id}
                  className="px-4 py-2 min-h-[44px] flex items-center justify-center rounded-md border text-sm font-medium hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  Set as Current
                </button>
                <Link 
                  href={`/projects/${project.id}/policies`} 
                  className="px-4 py-2 min-h-[44px] flex items-center justify-center rounded-md border text-sm font-medium hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 transition-colors"
                >
                  Manage Policies
                </Link>
              </div>
            </li>
          ))}
        </ul>
      )}

      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>New project</DialogTitle>
            <DialogDescription>
              A project is a locked drawer for one system&apos;s secrets, policies and providers.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="proj-name">Name</Label>
              <Input
                id="proj-name"
                value={projName}
                autoFocus
                onChange={e => {
                  setProjName(e.target.value)
                  if (!slugTouched) setProjSlug(slugify(e.target.value))
                }}
                placeholder="e.g. platform-prod"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="proj-slug">Slug</Label>
              <Input
                id="proj-slug"
                value={projSlug}
                onChange={e => { setSlugTouched(true); setProjSlug(e.target.value) }}
                placeholder="platform-prod"
              />
            </div>
          </div>
          <DialogFooter className="pt-2">
            <Button variant="outline" onClick={() => setShowCreate(false)}>Cancel</Button>
            <Button disabled={creating || !projName.trim() || !projSlug.trim()} onClick={() => void handleCreate()}>
              {creating ? 'Creating…' : 'Create project'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
