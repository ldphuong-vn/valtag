'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useEffect, useState } from 'react'
import { KeyRound, ClipboardCheck, ScrollText, Settings, LogOut, Shield, Bot, ScanLine, Database, ShieldCheck, FolderOpen, ShieldAlert, LayoutDashboard, type LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

// IA modeled on the entity-first sidebars of Vault UI (Secrets/Access/Tools),
// 1Password Business (People/Vaults/Policies/Reports) and Tailscale
// (Machines/Users/Access controls): primary objects first, policy and
// operations as their own labeled groups — never buried under Settings.
// With the single-org policy (2026-11-21) there is no org switcher here:
// the org lives on the Overview page.
const navGroups: { label: string; items: { href: string; label: string; icon: LucideIcon }[] }[] = [
  {
    label: 'Day-to-day',
    items: [
      { href: '/overview', label: 'Overview', icon: LayoutDashboard },
      { href: '/secrets', label: 'Secrets', icon: KeyRound },
      { href: '/approvals', label: 'Approvals', icon: ClipboardCheck },
    ],
  },
  {
    label: 'Access & policies',
    items: [
      { href: '/agents', label: 'Agents', icon: Bot },
      { href: '/projects', label: 'Projects', icon: FolderOpen },
    ],
  },
  {
    label: 'Credential engines',
    items: [
      { href: '/providers', label: 'Providers', icon: Database },
      { href: '/scans', label: 'Secret Scanner', icon: ScanLine },
    ],
  },
  {
    label: 'Operations',
    items: [
      { href: '/audit', label: 'Audit', icon: ScrollText },
      { href: '/settings', label: 'Settings', icon: Settings },
    ],
  },
]

async function handleLogout() {
  await fetch('/api/auth/logout', { method: 'POST' })
  window.location.href = '/login'
}

export default function Sidebar() {
  const pathname = usePathname()
  const [policyHref, setPolicyHref] = useState('/projects')

  useEffect(() => {
    const updateHref = () => {
      const pid = localStorage.getItem('valt_current_project')
      setPolicyHref(pid ? `/projects/${pid}/policies` : '/projects')
    }
    updateHref()
    // Self-heal: a returning user whose localStorage was cleared (new browser,
    // cleared site data) otherwise loses their context — re-derive it from
    // their first org/workspace/project.
    async function ensureContext() {
      const wsId = localStorage.getItem('valt_current_workspace')
      const hasProject = !!localStorage.getItem('valt_current_project')
      if (wsId && hasProject) return
      try {
        let orgId = localStorage.getItem('valt_current_org')
        let workspaceId = wsId
        if (!orgId) {
          const orgsRes = await fetch('/api/proxy/orgs', { credentials: 'include' })
          const orgsData = await orgsRes.json()
          const orgs = Array.isArray(orgsData) ? orgsData : (orgsData.orgs ?? [])
          orgId = orgs[0]?.id ?? null
          if (!orgId) return
          localStorage.setItem('valt_current_org', orgId)
        }
        if (!workspaceId) {
          const wsRes = await fetch(`/api/proxy/orgs/${orgId}/workspaces`, { credentials: 'include' })
          const wsData = await wsRes.json()
          const workspaces = Array.isArray(wsData) ? wsData : (wsData.workspaces ?? [])
          const ws = workspaces[0]
          if (!ws) return
          workspaceId = ws.id as string
          const wsName = ws.name ?? 'Workspace'
          localStorage.setItem('valt_current_workspace', workspaceId)
          localStorage.setItem('valt_current_workspace_name', wsName as string)
        }
        if (!hasProject) {
          const projRes = await fetch(`/api/proxy/workspaces/${workspaceId}/projects`, { credentials: 'include' })
          const projData = await projRes.json()
          const projects = Array.isArray(projData) ? projData : (projData.projects ?? [])
          if (projects[0]?.id) {
            localStorage.setItem('valt_current_project', projects[0].id as string)
          }
        }
        updateHref()
        window.dispatchEvent(new Event('valtag:workspace-changed'))
        window.dispatchEvent(new Event('valtag:project-changed'))
      } catch {
        // stay without context — Overview shows what is missing
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

  const groups = navGroups.map(g => ({
    ...g,
    // Policies belong to the current project (entry from its own group, next
    // to Projects — not buried after Settings).
    items: g.label === 'Access & policies'
      ? [...g.items, { href: policyHref, label: 'Policies', icon: ShieldCheck as LucideIcon }]
      : g.items,
  }))

  return (
    <aside className="flex flex-col w-60 border-r bg-card shrink-0">
      <div className="flex items-center gap-2 px-4 h-14 border-b">
        <div className="w-7 h-7 rounded-lg bg-primary/10 flex items-center justify-center">
          <Shield className="w-4 h-4 text-primary" />
        </div>
        <span className="font-semibold text-sm">valtag</span>
      </div>

      <nav className="flex-1 px-3 py-3 overflow-y-auto space-y-3">
        {groups.map(group => (
          <div key={group.label}>
            <div className="px-3 pb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/60">
              {group.label}
            </div>
            <div className="space-y-0.5">
              {group.items.map(({ href, label, icon: Icon }) => (
                <Link
                  key={`${href}-${label}`}
                  href={href}
                  className={cn(
                    'flex items-center gap-3 px-3 py-1.5 rounded-md text-sm font-medium transition-colors',
                    (label === 'Policies' ? pathname.includes('/policies') : pathname.startsWith(href))
                      ? 'bg-primary/10 text-primary'
                      : 'text-muted-foreground hover:bg-accent hover:text-foreground',
                  )}
                >
                  <Icon className="w-4 h-4 shrink-0" />
                  {label}
                </Link>
              ))}
            </div>
          </div>
        ))}
      </nav>

      {/* Admin + Sign out share one icon row to keep the sidebar scroll-free;
          the title attribute shows the label on hover. */}
      <div className="px-3 py-2.5 border-t flex items-center gap-1">
        <Link
          href="/admin"
          title="Admin"
          className={cn(
            'flex-1 h-9 rounded-md grid place-items-center transition-colors',
            pathname.startsWith('/admin')
              ? 'bg-primary/10 text-primary'
              : 'text-muted-foreground hover:bg-accent hover:text-foreground',
          )}
        >
          <ShieldAlert className="w-4 h-4" />
        </Link>
        <button
          onClick={handleLogout}
          title="Sign out"
          className="flex-1 h-9 rounded-md grid place-items-center text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
        >
          <LogOut className="w-4 h-4" />
        </button>
      </div>
    </aside>
  )
}
