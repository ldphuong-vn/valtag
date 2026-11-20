'use client'

import { useState, useEffect, useCallback } from 'react'
import Link from 'next/link'
import { useParams } from 'next/navigation'
import { api } from '@/lib/api-client'
import type { DynamicLease } from '@/types/api'
import { Button } from '@/components/ui/button'

// Lease list for one dynamic provider — the page behind the "View Leases"
// link on the Providers list. Server API: GET /providers/{id}/leases
// (active leases only — expired/revoked leases age out of this list).
export default function ProviderLeasesPage() {
  const params = useParams<{ id: string }>()
  const providerId = params?.id ?? ''

  const [leases, setLeases] = useState<DynamicLease[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busyLease, setBusyLease] = useState<string | null>(null)

  const fetchLeases = useCallback(async () => {
    if (!providerId) return
    setLoading(true)
    setError(null)
    try {
      const res = await api.providers.listLeases(providerId)
      setLeases(res.leases ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load leases')
    } finally {
      setLoading(false)
    }
  }, [providerId])

  useEffect(() => {
    void fetchLeases()
  }, [fetchLeases])

  async function revoke(leaseId: string) {
    setBusyLease(leaseId)
    try {
      await api.providers.revokeLease(leaseId)
      await fetchLeases()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke lease')
    } finally {
      setBusyLease(null)
    }
  }

  function fmt(iso: string) {
    try {
      return new Date(iso).toLocaleString()
    } catch {
      return iso
    }
  }

  function remaining(expiresAt: string) {
    const ms = new Date(expiresAt).getTime() - Date.now()
    if (ms <= 0) return 'expiring…'
    const mins = Math.ceil(ms / 60000)
    return mins >= 60 ? `${Math.floor(mins / 60)}h ${mins % 60}m left` : `${mins}m left`
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Provider Leases</h1>
          <p className="text-sm text-muted-foreground mt-1 font-mono">
            {providerId}
          </p>
          <p className="text-sm text-muted-foreground mt-1">
            Credentials currently minted by this provider. Each lease dies on its
            own expiry; revoke kills it immediately.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => void fetchLeases()}>🔄 Refresh</Button>
          <Link href="/providers">
            <Button variant="ghost">← Providers</Button>
          </Link>
        </div>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading leases…</p>
      ) : leases.length === 0 ? (
        <div className="rounded-lg border bg-card p-10 text-center text-sm text-muted-foreground">
          No active leases. Mint one from an agent (request access) or the CLI —
          they will appear here until they expire.
        </div>
      ) : (
        <div className="rounded-lg border bg-card overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40">
                <th className="px-4 py-3 text-left font-medium">Lease ID</th>
                <th className="px-4 py-3 text-left font-medium">Agent</th>
                <th className="px-4 py-3 text-left font-medium">TTL</th>
                <th className="px-4 py-3 text-left font-medium">Expires</th>
                <th className="px-4 py-3 text-left font-medium">Created</th>
                <th className="px-4 py-3 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {leases.map(l => (
                <tr key={l.id} className="border-b last:border-0 hover:bg-muted/20 transition-colors">
                  <td className="px-4 py-3 font-mono text-xs">{l.id.slice(0, 8)}…</td>
                  <td className="px-4 py-3 text-muted-foreground">{l.agent_id ? `${l.agent_id.slice(0, 8)}…` : '—'}</td>
                  <td className="px-4 py-3">{l.ttl_seconds >= 60 ? `${Math.round(l.ttl_seconds / 60)}m` : `${l.ttl_seconds}s`}</td>
                  <td className="px-4 py-3">
                    <span className="inline-flex items-center gap-1.5">
                      <span className="inline-block w-1.5 h-1.5 rounded-full bg-green-500" />
                      {remaining(l.expires_at)}
                    </span>
                    <span className="block text-xs text-muted-foreground">{fmt(l.expires_at)}</span>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground text-xs">{fmt(l.created_at)}</td>
                  <td className="px-4 py-3 text-right">
                    <Button
                      size="sm"
                      variant="destructive"
                      disabled={busyLease === l.id}
                      onClick={() => void revoke(l.id)}
                    >
                      {busyLease === l.id ? 'Revoking…' : 'Revoke'}
                    </Button>
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
