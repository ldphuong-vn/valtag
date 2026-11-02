'use client'

import { useEffect, useState } from 'react'
import { Shield, Building2, FolderOpen, CheckCircle2, ArrowRight, ChevronRight } from 'lucide-react'
import { api } from '@/lib/api-client'
import type { Organization, Workspace } from '@/types/api'

// Steps: org-confirm → project-create → done
type Step = 'loading' | 'org-confirm' | 'project-create' | 'done'

export default function OnboardingPage() {
  const [step, setStep] = useState<Step>('loading')
  const [org, setOrg] = useState<Organization | null>(null)
  const [workspace, setWorkspace] = useState<Workspace | null>(null)
  const [orgName, setOrgName] = useState('')
  const [projectName, setProjectName] = useState('')
  const [projectSlug, setProjectSlug] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // On mount: fetch user's first org + default workspace.
  // If they already have a project, skip to dashboard (idempotent).
  useEffect(() => {
    async function bootstrap() {
      try {
        const { orgs } = await api.orgs.list()
        if (orgs.length === 0) {
          // No org yet — shouldn't happen after auto-create, but handle gracefully.
          setStep('org-confirm')
          return
        }

        const firstOrg = orgs[0]
        setOrg(firstOrg)
        setOrgName(firstOrg.name)

        // Fetch workspaces for the org
        const { workspaces } = await api.workspaces.list(firstOrg.id)
        const defaultWs = workspaces[0] ?? null
        setWorkspace(defaultWs)

        if (defaultWs) {
          // Check if any projects already exist — if so, skip onboarding.
          const { projects } = await api.projects.list(defaultWs.id)
          if (projects.length > 0) {
            // Already onboarded — set workspace context and redirect.
            persistWorkspaceContext(firstOrg.id, defaultWs.id, defaultWs.name, projects[0].id)
            window.location.href = '/'
            return
          }
        }

        setStep('org-confirm')
      } catch {
        // On any error just show the first step rather than blocking.
        setStep('org-confirm')
      }
    }

    void bootstrap()
  }, [])

  // Persist workspace + project selection into localStorage (matches app conventions).
  function persistWorkspaceContext(orgId: string, workspaceId: string, workspaceName: string, projectId?: string) {
    localStorage.setItem('valt_current_org', orgId)
    localStorage.setItem('valt_current_workspace', workspaceId)
    localStorage.setItem('valt_current_workspace_name', workspaceName)
    if (projectId) localStorage.setItem('valt_current_project', projectId)
  }

  async function handleConfirmOrg(e: React.FormEvent) {
    e.preventDefault()
    if (!org) { setStep('project-create'); return }
    setError(null)
    // Only update if name changed
    if (orgName.trim() !== org.name) {
      setSubmitting(true)
      try {
        // Use raw fetch via proxy — org update endpoint
        const res = await fetch(`/api/proxy/orgs/${org.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: orgName.trim() }),
        })
        if (!res.ok) {
          const err = await res.json().catch(() => ({}))
          setError((err as { error?: string }).error ?? 'Failed to update org name')
          return
        }
      } catch {
        setError('Network error updating org name')
        return
      } finally {
        setSubmitting(false)
      }
    }
    setStep('project-create')
  }

  async function handleCreateProject(e: React.FormEvent) {
    e.preventDefault()
    if (!workspace) { setStep('done'); return }
    setError(null)
    setSubmitting(true)
    try {
      const project = await api.projects.create(workspace.id, {
        name: projectName.trim(),
        slug: projectSlug.trim(),
      })
      // Persist context so user lands on their new project immediately.
      if (org) {
        persistWorkspaceContext(org.id, workspace.id, workspace.name, project.id)
        window.dispatchEvent(new Event('valtag:workspace-changed'))
        window.dispatchEvent(new Event('valtag:project-changed'))
      }
      setStep('done')
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  function handleProjectNameChange(e: React.ChangeEvent<HTMLInputElement>) {
    const name = e.target.value
    const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
    setProjectName(name)
    setProjectSlug(slug)
  }

  // --- Step progress indicator ---
  const steps = ['Organization', 'Project', 'Done']
  const stepIndex = step === 'loading' ? 0 : step === 'org-confirm' ? 0 : step === 'project-create' ? 1 : 2

  if (step === 'loading') {
    return (
      <div className="flex flex-col items-center justify-center gap-4 py-20">
        <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center">
          <Shield className="w-5 h-5 text-primary" />
        </div>
        <p className="text-sm text-muted-foreground animate-pulse">Setting up your account…</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-8">
      {/* Header */}
      <div className="text-center">
        <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-primary/10 mb-4">
          <Shield className="w-6 h-6 text-primary" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight">Welcome to valtag</h1>
        <p className="text-sm text-muted-foreground mt-1">Let's get you set up in a minute</p>
      </div>

      {/* Progress */}
      <div className="flex items-center justify-center gap-2">
        {steps.map((label, i) => (
          <div key={label} className="flex items-center gap-2">
            <div className={`flex items-center gap-1.5 ${i <= stepIndex ? 'text-primary' : 'text-muted-foreground'}`}>
              <div className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-semibold border-2 transition-colors ${
                i < stepIndex
                  ? 'bg-primary border-primary text-primary-foreground'
                  : i === stepIndex
                  ? 'border-primary text-primary'
                  : 'border-muted-foreground/30 text-muted-foreground'
              }`}>
                {i < stepIndex ? <CheckCircle2 className="w-3.5 h-3.5" /> : i + 1}
              </div>
              <span className="text-xs font-medium hidden sm:inline">{label}</span>
            </div>
            {i < steps.length - 1 && (
              <ChevronRight className="w-4 h-4 text-muted-foreground/40" />
            )}
          </div>
        ))}
      </div>

      {/* Step cards */}
      <div className="rounded-2xl border bg-card shadow-lg p-8">
        {error && (
          <div className="mb-6 p-3 rounded-lg border border-destructive/50 bg-destructive/10 text-sm text-destructive">
            {error}
          </div>
        )}

        {/* Step 1: Confirm org name */}
        {step === 'org-confirm' && (
          <form onSubmit={handleConfirmOrg} className="space-y-6">
            <div className="flex items-center gap-3 mb-2">
              <div className="w-9 h-9 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
                <Building2 className="w-5 h-5 text-primary" />
              </div>
              <div>
                <h2 className="text-lg font-semibold">Your Organization</h2>
                <p className="text-xs text-muted-foreground">We created one for you — feel free to rename it</p>
              </div>
            </div>

            <div>
              <label htmlFor="org-name" className="block text-sm font-medium mb-1.5">
                Organization name
              </label>
              <input
                id="org-name"
                type="text"
                required
                value={orgName}
                onChange={e => setOrgName(e.target.value)}
                placeholder="My Organization"
                className="w-full px-3 py-2.5 text-sm border rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>

            <div className="flex gap-3 pt-2">
              <button
                type="submit"
                disabled={submitting || !orgName.trim()}
                className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
              >
                {submitting ? 'Saving…' : 'Continue'}
                {!submitting && <ArrowRight className="w-4 h-4" />}
              </button>
              <button
                type="button"
                onClick={() => setStep('project-create')}
                className="px-4 py-2.5 rounded-lg text-sm text-muted-foreground hover:bg-accent transition-colors"
              >
                Skip
              </button>
            </div>
          </form>
        )}

        {/* Step 2: Create first project */}
        {step === 'project-create' && (
          <form onSubmit={handleCreateProject} className="space-y-6">
            <div className="flex items-center gap-3 mb-2">
              <div className="w-9 h-9 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
                <FolderOpen className="w-5 h-5 text-primary" />
              </div>
              <div>
                <h2 className="text-lg font-semibold">Create your first project</h2>
                <p className="text-xs text-muted-foreground">Projects group secrets and agents together</p>
              </div>
            </div>

            {!workspace && (
              <div className="p-3 rounded-lg border border-amber-500/30 bg-amber-500/10 text-sm text-amber-700 dark:text-amber-400">
                No workspace found. You can create projects from the Projects page after setup.
              </div>
            )}

            <div>
              <label htmlFor="project-name" className="block text-sm font-medium mb-1.5">
                Project name
              </label>
              <input
                id="project-name"
                type="text"
                required={!!workspace}
                value={projectName}
                onChange={handleProjectNameChange}
                placeholder="My Project"
                className="w-full px-3 py-2.5 text-sm border rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-primary"
                disabled={!workspace}
              />
            </div>

            {projectSlug && (
              <p className="text-xs text-muted-foreground -mt-4">
                Slug: <span className="font-mono">{projectSlug}</span>
              </p>
            )}

            <div className="flex gap-3 pt-2">
              <button
                type="submit"
                disabled={submitting || (!projectName.trim() && !!workspace)}
                className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
              >
                {submitting ? 'Creating…' : 'Create Project'}
                {!submitting && <ArrowRight className="w-4 h-4" />}
              </button>
              <button
                type="button"
                onClick={() => setStep('done')}
                className="px-4 py-2.5 rounded-lg text-sm text-muted-foreground hover:bg-accent transition-colors"
              >
                Skip
              </button>
            </div>
          </form>
        )}

        {/* Step 3: Done */}
        {step === 'done' && (
          <div className="text-center space-y-6">
            <div className="flex justify-center">
              <div className="w-16 h-16 rounded-full bg-emerald-500/10 flex items-center justify-center">
                <CheckCircle2 className="w-8 h-8 text-emerald-500" />
              </div>
            </div>
            <div>
              <h2 className="text-xl font-semibold mb-2">You're all set!</h2>
              <p className="text-sm text-muted-foreground max-w-sm mx-auto">
                Your organization and workspace are ready. Start adding secrets and connecting your AI agents.
              </p>
            </div>
            <button
              onClick={() => { window.location.href = '/' }}
              className="inline-flex items-center gap-2 px-6 py-2.5 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
            >
              Go to Dashboard
              <ArrowRight className="w-4 h-4" />
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
