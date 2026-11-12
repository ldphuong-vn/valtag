'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useEffect, useState } from 'react'
import { KeyRound, ClipboardCheck, ScrollText, Settings, LogOut, Shield, Bot, Building2, ChevronDown, ScanLine, Database, Zap, Bell, ShieldCheck, Layers, FolderOpen, ShieldAlert } from 'lucide-react'
import { cn } from '@/lib/utils'

const navItems = [
  { href: '/secrets', label: 'Secrets', icon: KeyRound },
  { href: '/approvals', label: 'Approvals', icon: ClipboardCheck },
  { href: '/audit', label: 'Audit', icon: ScrollText },
  { href: '/agents', label: 'Agents', icon: Bot },
  { href: '/scans', label: 'Scanner', icon: ScanLine },
  { href: '/providers', label: 'Providers', icon: Database },
  { href: '/projects', label: 'Projects', icon: FolderOpen },
  { href: '/settings', label: 'Settings', icon: Settings },
]

async function handleLogout() {
  await fetch('/api/auth/logout', { method: 'POST' })
  window.location.href = '/login'
}

export default function Sidebar() {
  const pathname = usePathname()
  const [policyHref, setPolicyHref] = useState('/projects')
  const [workspaceName, setWorkspaceName] = useState<string | null>(null)

  useEffect(() => {
    const updateHref = () => {
      const pid = localStorage.getItem('valt_current_project')
      setPolicyHref(pid ? `/projects/${pid}/policies` : '/projects')

      const wsId = localStorage.getItem('valt_current_workspace')
      const wsName = localStorage.getItem('valt_current_workspace_name')
      setWorkspaceName(wsId ? (wsName || 'Workspace selected') : null)
    }
    updateHref()
    // Self-heal: a returning user whose localStorage was cleared (new browser,
    // cleared site data) otherwise sees "No workspace selected" forever —
    // re-derive the context from their first org/workspace/project.
    async function ensureContext() {
      if (localStorage.getItem('valt_current_workspace')) return
      try {
        const orgsRes = await fetch('/api/proxy/orgs', { credentials: 'include' })
        const orgsData = await orgsRes.json()
        const orgs = Array.isArray(orgsData) ? orgsData : (orgsData.orgs ?? [])
        const org = orgs[0]
        if (!org) return
        const wsRes = await fetch(`/api/proxy/orgs/${org.id}/workspaces`, { credentials: 'include' })
        const wsData = await wsRes.json()
        const workspaces = Array.isArray(wsData) ? wsData : (wsData.workspaces ?? [])
        const ws = workspaces[0]
        if (!ws) return
        localStorage.setItem('valt_current_org', org.id)
        localStorage.setItem('valt_current_workspace', ws.id)
        localStorage.setItem('valt_current_workspace_name', ws.name ?? 'Workspace')
        let projectId: string | null = null
        const projRes = await fetch(`/api/proxy/workspaces/${ws.id}/projects`, { credentials: 'include' })
        const projData = await projRes.json()
        const projects = Array.isArray(projData) ? projData : (projData.projects ?? [])
        if (projects[0]?.id) {
          projectId = projects[0].id
          localStorage.setItem('valt_current_project', projectId as string)
        }
        if (!projectId) return
        updateHref()
        window.dispatchEvent(new Event('valtag:workspace-changed'))
        if (projectId) window.dispatchEvent(new Event('valtag:project-changed'))
      } catch {
        // stay on the "No workspace selected" state — user can pick via /orgs
      }
    }
    void ensureContext()
    window.addEventListener('storage', updateHref)
    window.addEventListener('valtag:project-changed', updateHref)
    window.addEventListener('valtag:workspace-changed', updateHref)
    return () => {
      window.removeEventListener('storage', updateHref)
      window.removeEventListener('valtag:project-changed', updateHref)
      window.removeEventListener('valtag:workspace-changed', updateHref)
    }
  }, [])

  const links = [...navItems, { href: policyHref, label: 'Policies', icon: ShieldCheck }]

  return (
    <aside className="flex flex-col w-60 border-r bg-card shrink-0">
      <div className="flex items-center gap-2 px-4 h-14 border-b">
        <div className="w-7 h-7 rounded-lg bg-primary/10 flex items-center justify-center">
          <Shield className="w-4 h-4 text-primary" />
        </div>
        <span className="font-semibold text-sm">valtag</span>
      </div>

      {/* Org/Workspace context */}
      <div className="px-3 py-3 border-b space-y-1">
        <Link
          href="/orgs"
          className="flex items-center gap-2 px-2 py-1.5 rounded-md text-xs font-medium text-muted-foreground hover:bg-accent hover:text-foreground transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <Building2 className="w-4 h-4 shrink-0" />
          <span className="truncate flex-1 text-left">Organizations</span>
          <ChevronDown className="w-3.5 h-3.5 shrink-0 opacity-50" />
        </Link>
        
        {workspaceName ? (
          <div className="flex items-center gap-2 px-2 py-1.5 rounded-md text-sm font-semibold bg-accent/50 text-foreground">
            <Layers className="w-4 h-4 shrink-0 text-primary" />
            <span className="truncate">{workspaceName}</span>
          </div>
        ) : (
          <div className="flex items-start gap-2 px-2 py-2 mt-1 rounded-md text-xs bg-amber-500/10 text-amber-600 border border-amber-500/20">
            <Layers className="w-4 h-4 shrink-0 mt-0.5" />
            <span className="leading-tight">No workspace selected. Go to Organizations to pick one.</span>
          </div>
        )}
      </div>

      <nav className="flex-1 p-3 space-y-1 overflow-y-auto">
        {links.map(({ href, label, icon: Icon }) => (
          <Link
            key={`${href}-${label}`}
            href={href}
            className={cn(
              'flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors',
              (label === 'Policies' ? pathname.includes('/policies') : pathname.startsWith(href))
                ? 'bg-primary/10 text-primary'
                : 'text-muted-foreground hover:bg-accent hover:text-foreground',
            )}
          >
            <Icon className="w-4 h-4 shrink-0" />
            {label}
          </Link>
        ))}
      </nav>

      <div className="p-3 border-t space-y-1">
        <Link
          href="/settings/notifications"
          className={cn(
            'flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors',
            pathname.startsWith('/settings/notifications')
              ? 'bg-primary/10 text-primary'
              : 'text-muted-foreground hover:bg-accent hover:text-foreground',
          )}
        >
          <Bell className="w-4 h-4 shrink-0" />
          Notifications
        </Link>
        <Link
          href="/settings/upgrade"
          className="flex items-center gap-3 px-3 py-2 rounded-md text-sm text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
        >
          <Zap className="w-4 h-4 shrink-0" />
          Upgrade
        </Link>
        <Link
          href="/admin"
          className={cn(
            'flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors',
            pathname.startsWith('/admin')
              ? 'bg-primary/10 text-primary'
              : 'text-muted-foreground hover:bg-accent hover:text-foreground',
          )}
        >
          <ShieldAlert className="w-4 h-4 shrink-0" />
          Admin
        </Link>
        <button
          onClick={handleLogout}
          className="flex items-center gap-3 px-3 py-2 rounded-md text-sm text-muted-foreground hover:bg-accent hover:text-foreground transition-colors w-full text-left"
        >
          <LogOut className="w-4 h-4 shrink-0" />
          Sign out
        </button>
      </div>
    </aside>
  )
}
