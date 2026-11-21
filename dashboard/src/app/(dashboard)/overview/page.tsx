'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { api } from '@/lib/api-client'
import type { AccessRequest, DynamicLease, DynamicProvider, Organization, Workspace } from '@/types/api'
import { Building2, Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

// Overview — the "who holds what right now" screen, plus the org home
// (single-org policy 2026-11-21: the /orgs pages folded in here).
// All data comes from APIs that already exist.
interface VerifyShape {
  valid: boolean
  checked: number
  head_hash: string
  head_mismatch?: boolean
}

interface ActiveLease {
  providerName: string
  lease: DynamicLease
}

interface OrgMember {
  id?: string
  user_id: string
  role: string
}

function slugify(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40)
}

function remaining(expiresAt: string): string {
  const ms = new Date(expiresAt).getTime() - Date.now()
  if (ms <= 0) return 'expiring…'
  const mins = Math.ceil(ms / 60000)
  return mins >= 60 ? `${Math.floor(mins / 60)}h ${mins % 60}m left` : `${mins}m left`
}

export default function OverviewPage() {
  const [pending, setPending] = useState<AccessRequest[] | null>(null)
  const [secretsCount, setSecretsCount] = useState<number | null>(null)
  const [leases, setLeases] = useState<ActiveLease[] | null>(null)
  const [verify, setVerify] = useState<VerifyShape | null>(null)
  const [verifyHidden, setVerifyHidden] = useState(false)

  // org home (single-org policy)
  const [org, setOrg] = useState<Organization | null>(null)
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [members, setMembers] = useState<OrgMember[]>([])
  const [currentWsId, setCurrentWsId] = useState('')
  const [showWsCreate, setShowWsCreate] = useState(false)
  const [wsName, setWsName] = useState('')
  const [wsSlug, setWsSlug] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [creatingWs, setCreatingWs] = useState(false)
  const [wsError, setWsError] = useState('')
  const [inviteOpen, setInviteOpen] = useState(false)
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRole, setInviteRole] = useState('member')
  const [inviteLink, setInviteLink] = useState('')
  const [inviteError, setInviteError] = useState('')
  const [inviting, setInviting] = useState(false)

  useEffect(() => {
    let alive = true
    async function load() {
      // org home — the single org this user belongs to
      try {
        const orgsRes = await api.orgs.list()
        const first = orgsRes.orgs[0] ?? null
        if (alive) setOrg(first)
        if (first) {
          const [wsRes, memRes] = await Promise.all([
            api.workspaces.list(first.id),
            api.orgs.getMembers(first.id).catch(() => ({ members: [] as OrgMember[] })),
          ])
          if (alive) {
            setWorkspaces(wsRes.workspaces ?? [])
            setMembers((memRes as { members?: OrgMember[] }).members ?? [])
          }
        } else if (alive) {
          setWorkspaces([])
          setMembers([])
        }
      } catch { if (alive) { setOrg(null); setWorkspaces([]) } }

      try {
        const r = await api.requests.list('pending')
        if (alive) setPending(r.requests ?? [])
      } catch { if (alive) setPending([]) }

      try {
        const s = await api.secrets.list()
        if (alive) setSecretsCount((s.secrets ?? []).length)
      } catch { if (alive) setSecretsCount(0) }

      try {
        const pid = localStorage.getItem('valt_current_project') ?? ''
        if (!pid) { if (alive) setLeases([]); return }
        const pr = await api.providers.list(pid)
        const provs: DynamicProvider[] = pr.providers ?? []
        const agg: ActiveLease[] = []
        for (const p of provs) {
          try {
            const lr = await api.providers.listLeases(p.id)
            for (const l of lr.leases ?? []) agg.push({ providerName: p.name, lease: l })
          } catch { /* provider leases endpoint unavailable for this one */ }
        }
        if (alive) setLeases(agg)
      } catch { if (alive) setLeases([]) }

      // Admin-only endpoint — hide the card for regular members.
      try {
        const r = await fetch('/api/proxy/audit/verify', { credentials: 'include' })
        if (!r.ok) { if (alive) { setVerify(null); setVerifyHidden(true) }; return }
        if (alive) setVerify(await r.json())
      } catch { if (alive) setVerifyHidden(true) }
    }
    void load()
    // context switch from anywhere must refresh every number on this page
    const reload = () => { alive = true; void load() }
    window.addEventListener('valtag:project-changed', reload)
    window.addEventListener('valtag:workspace-changed', reload)
    window.addEventListener('storage', reload)
    return () => {
      alive = false
      window.removeEventListener('valtag:project-changed', reload)
      window.removeEventListener('valtag:workspace-changed', reload)
      window.removeEventListener('storage', reload)
    }
  }, [])

  function selectWorkspace(ws: Workspace) {
    if (!org) return
    localStorage.setItem('valt_current_org', org.id)
    localStorage.setItem('valt_current_workspace', ws.id)
    localStorage.setItem('valt_current_workspace_name', ws.name)
    // auto-pick the workspace's first project so the user never lands in a
    // context with no project selected
    fetch(`/api/proxy/workspaces/${ws.id}/projects`, { credentials: 'include' })
      .then(r => r.json())
      .then(d => {
        const projects = Array.isArray(d) ? d : (d.projects ?? [])
        if (projects[0]?.id) localStorage.setItem('valt_current_project', projects[0].id as string)
        else localStorage.removeItem('valt_current_project')
        window.dispatchEvent(new Event('valtag:workspace-changed'))
        window.dispatchEvent(new Event('valtag:project-changed'))
      })
      .catch(() => {
        window.dispatchEvent(new Event('valtag:workspace-changed'))
        window.dispatchEvent(new Event('valtag:project-changed'))
      })
    setCurrentWsId(ws.id)
  }

  async function createWorkspace() {
    if (!org || !wsName.trim() || !wsSlug.trim()) return
    setCreatingWs(true)
    setWsError('')
    try {
      const created = await api.workspaces.create(org.id, { name: wsName.trim(), slug: wsSlug.trim() })
      setShowWsCreate(false)
      setWsName(''); setWsSlug(''); setSlugTouched(false)
      if (created?.id) {
        setWorkspaces(w => [...w, created])
        selectWorkspace(created)
      }
    } catch (err) {
      setWsError(err instanceof Error ? err.message : 'Failed to create workspace')
    } finally {
      setCreatingWs(false)
    }
  }

  async function inviteMember() {
    if (!org || !inviteEmail.trim()) return
    setInviting(true)
    setInviteLink('')
    setInviteError('')
    try {
      const res = await api.orgs.invite(org.id, { email: inviteEmail.trim(), role: inviteRole })
      setInviteLink(res.invite_url)
      setInviteEmail('')
    } catch (err) {
      setInviteError(err instanceof Error ? err.message : 'Failed to create invitation')
    } finally {
      setInviting(false)
    }
  }

  const stat = (label: string, value: string | number | null, href: string, accent: string) => (
    <Link href={href} className="rounded-xl border bg-card p-4 hover:border-primary/50 transition-colors">
      <div className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{label}</div>
      <div className={`text-2xl font-bold mt-1 ${accent}`}>{value ?? '…'}</div>
    </Link>
  )

  const verifyCard = verifyHidden ? null : (
    <Link href="/audit" className="rounded-xl border bg-card p-4 hover:border-primary/50 transition-colors">
      <div className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Audit chain</div>
      <div className={`text-2xl font-bold mt-1 ${verify ? (verify.valid ? 'text-green-500' : 'text-red-500') : ''}`}>
        {verify ? (verify.valid ? '✓ valid' : '✗ broken') : '…'}
      </div>
      <div className="text-xs text-muted-foreground mt-1">
        {verify ? `${verify.checked} entries · head ${verify.head_hash.slice(0, 10)}…` : 'checking…'}
      </div>
    </Link>
  )

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Overview</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Who holds what right now — approvals waiting, active leases, and the health of the audit chain.
        </p>
      </div>

      {/* Your organization — folded in here (single-org policy, /orgs removed) */}
      {org && (
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between gap-3">
              <CardTitle className="text-base flex items-center gap-2">
                <Building2 className="w-4 h-4 text-muted-foreground" /> {org.name}
              </CardTitle>
            </div>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-2">
              {workspaces.map(ws => (
                <div key={ws.id} className="flex items-center justify-between rounded-md border px-3 py-2">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium truncate">{ws.name}</span>
                      {currentWsId === ws.id && (
                        <Badge variant="secondary" className="text-green-600 dark:text-green-400 border border-green-600/30 bg-green-500/10">
                          current
                        </Badge>
                      )}
                    </div>
                    <p className="text-xs text-muted-foreground font-mono">{ws.slug}</p>
                  </div>
                  <Button
                    size="sm"
                    variant={currentWsId === ws.id ? 'ghost' : 'outline'}
                    disabled={currentWsId === ws.id}
                    onClick={() => selectWorkspace(ws)}
                  >
                    {currentWsId === ws.id ? 'Current' : 'Set current'}
                  </Button>
                </div>
              ))}
            </div>
            <Button size="sm" variant="outline" onClick={() => { setShowWsCreate(true); setWsName(''); setWsSlug(''); setSlugTouched(false); setWsError('') }}>
              <Plus className="w-4 h-4" /> New workspace
            </Button>

            <div className="border-t pt-3">
              <div className="flex items-center justify-between mb-2">
                <div className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  Members ({members.length})
                </div>
                <Button size="sm" variant="outline" onClick={() => { setInviteOpen(true); setInviteEmail(''); setInviteRole('member'); setInviteLink(''); setInviteError('') }}>
                  Invite member
                </Button>
              </div>
              <div className="space-y-1">
                {members.map(m => (
                  <div key={`${m.user_id}-${m.role}`} className="flex items-center justify-between text-sm">
                    <span className="font-mono text-xs text-muted-foreground">{m.user_id.slice(0, 8)}…</span>
                    <Badge variant="secondary" className="capitalize">{m.role}</Badge>
                  </div>
                ))}
                {members.length === 0 && <p className="text-xs text-muted-foreground">No members listed.</p>}
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Invite member dialog — creates an invitation and shows the link to
          hand over (email delivery needs SMTP; copy works without it). */}
      {inviteOpen && org && (
        <Card className="fixed inset-x-4 top-16 z-50 max-w-md mx-auto shadow-xl">
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Invite member to {org.name}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="ov-invite-email">Email</Label>
              <Input
                id="ov-invite-email"
                value={inviteEmail}
                autoFocus
                type="email"
                onChange={e => setInviteEmail(e.target.value)}
                placeholder="teammate@company.com"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="ov-invite-role">Role</Label>
              <select
                id="ov-invite-role"
                value={inviteRole}
                onChange={e => setInviteRole(e.target.value)}
                className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
              >
                <option value="member">member</option>
                <option value="admin">admin</option>
              </select>
            </div>
            {inviteLink && (
              <div className="space-y-1.5">
                <Label>Invitation link — copy and hand over to the invitee</Label>
                <Input readOnly value={inviteLink} className="font-mono text-xs" onFocus={e => e.currentTarget.select()} />
              </div>
            )}
            {inviteError && <p className="text-sm text-destructive">{inviteError}</p>}
            <div className="flex items-center gap-2">
              <Button size="sm" disabled={inviting || !inviteEmail.trim()} onClick={() => void inviteMember()}>
                {inviting ? 'Creating…' : inviteLink ? 'Create another' : 'Create invitation'}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setInviteOpen(false)}>Close</Button>
            </div>
          </CardContent>
        </Card>
      )}

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        {stat('Pending approvals', pending?.length ?? null, '/approvals', pending?.length ? 'text-orange-500' : 'text-green-500')}
        {stat('Active leases', leases?.length ?? null, '/providers', 'text-blue-500')}
        {stat('Secrets', secretsCount, '/secrets', '')}
        {verifyCard ?? stat('Audit chain', 'admin only', '/audit', 'text-muted-foreground')}
      </div>

      <div className="grid lg:grid-cols-2 gap-6">
        {/* Waiting for approval */}
        <div className="rounded-xl border bg-card">
          <div className="flex items-center justify-between px-4 py-3 border-b">
            <h2 className="text-sm font-semibold">Waiting for approval</h2>
            <Link href="/approvals" className="text-xs text-primary hover:underline">Approvals →</Link>
          </div>
          <div className="p-3 space-y-2">
            {pending === null ? (
              <p className="text-sm text-muted-foreground">Loading…</p>
            ) : pending.length === 0 ? (
              <p className="text-sm text-muted-foreground">Nothing waiting — good.</p>
            ) : (
              pending.slice(0, 4).map(r => (
                <div key={r.id} className="rounded-lg border px-3 py-2.5">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-sm font-medium truncate">{r.secret_name ?? r.secret_id.slice(0, 8)}</span>
                    <span className="text-xs text-orange-500 font-semibold shrink-0">{r.requested_duration_minutes}m asked</span>
                  </div>
                  <div className="text-xs text-muted-foreground truncate mt-0.5">
                    {r.ai_agent_id ? `agent ${r.ai_agent_id.slice(0, 8)}…` : 'user request'}
                    {r.reason ? ` · “${r.reason}”` : ''}
                  </div>
                </div>
              ))
            )}
            {pending !== null && pending.length > 4 && (
              <Link href="/approvals" className="text-xs text-primary hover:underline">
                +{pending.length - 4} more — review in Approvals
              </Link>
            )}
          </div>
        </div>

        {/* Active leases */}
        <div className="rounded-xl border bg-card">
          <div className="flex items-center justify-between px-4 py-3 border-b">
            <h2 className="text-sm font-semibold">Active leases</h2>
            <Link href="/providers" className="text-xs text-primary hover:underline">Providers →</Link>
          </div>
          <div className="p-3 space-y-2">
            {leases === null ? (
              <p className="text-sm text-muted-foreground">Loading…</p>
            ) : leases.length === 0 ? (
              <p className="text-sm text-muted-foreground">No credentials currently minted.</p>
            ) : (
              leases.slice(0, 5).map(({ providerName, lease }) => (
                <div key={lease.id} className="rounded-lg border px-3 py-2.5 flex items-center justify-between gap-2">
                  <div className="min-w-0">
                    <div className="text-sm font-medium truncate">{providerName}</div>
                    <div className="text-xs text-muted-foreground truncate">
                      {lease.agent_id ? `agent ${lease.agent_id.slice(0, 8)}…` : 'user lease'}
                    </div>
                  </div>
                  <span className="text-xs text-blue-500 font-semibold shrink-0">{remaining(lease.expires_at)}</span>
                </div>
              ))
            )}
          </div>
        </div>
      </div>

      {/* New workspace dialog */}
      <Dialog open={showWsCreate} onOpenChange={setShowWsCreate}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>New workspace</DialogTitle>
            <DialogDescription>
              A workspace groups related projects inside {org?.name ?? 'the organization'}.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="ov-ws-name">Name</Label>
              <Input
                id="ov-ws-name"
                value={wsName}
                autoFocus
                onChange={e => {
                  setWsName(e.target.value)
                  if (!slugTouched) setWsSlug(slugify(e.target.value))
                }}
                placeholder="e.g. platform"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="ov-ws-slug">Slug</Label>
              <Input
                id="ov-ws-slug"
                value={wsSlug}
                onChange={e => { setSlugTouched(true); setWsSlug(e.target.value) }}
                placeholder="platform"
              />
            </div>
          </div>
          <DialogFooter className="pt-2">
            <Button variant="outline" onClick={() => setShowWsCreate(false)}>Cancel</Button>
            <Button disabled={creatingWs || !wsName.trim() || !wsSlug.trim()} onClick={() => void createWorkspace()}>
              {creatingWs ? 'Creating…' : 'Create workspace'}
            </Button>
          </DialogFooter>
          {wsError && <p className="text-sm text-destructive">{wsError}</p>}
        </DialogContent>
      </Dialog>
    </div>
  )
}
