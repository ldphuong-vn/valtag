'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { api } from '@/lib/api-client'
import type { AccessRequest, DynamicLease, DynamicProvider } from '@/types/api'

// Overview — the "who holds what right now" screen. All data comes from
// APIs that already exist: pending access requests, secrets count,
// per-provider active leases, and (admin-only) the audit chain check.
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

  useEffect(() => {
    let alive = true
    async function load() {
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
    // context switch from the sidebar must refresh every number on this page
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
    </div>
  )
}
