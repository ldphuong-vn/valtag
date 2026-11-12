'use client'

import { useEffect, useState } from 'react'
import { api } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import type { AccessRequest } from '@/types/api'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import ApprovalActions from './approval-actions'

const TABS = ['all', 'pending', 'approved', 'rejected'] as const
type Tab = typeof TABS[number]

const statusVariant = (s: string) => {
  if (s === 'approved') return 'default'
  if (s === 'rejected') return 'destructive'
  return 'secondary'
}

export default function ApprovalList() {
  const [requests, setRequests] = useState<AccessRequest[]>([])
  const [tab, setTab] = useState<Tab>('all')
  const [loading, setLoading] = useState(true)
  const [actionsFor, setActionsFor] = useState<AccessRequest | null>(null)
  // (d) dogfood: approve/reject left an empty filtered table with no feedback.
  const [notice, setNotice] = useState('')
  // (e) dogfood: approved requests had no way to view the granted credential.
  const [credentialFor, setCredentialFor] = useState<AccessRequest | null>(null)
  const [credential, setCredential] = useState<{ value: string; status: string; expires_at?: string } | null>(null)
  const [credentialLoading, setCredentialLoading] = useState(false)
  const [credentialError, setCredentialError] = useState('')

  async function load() {
    setLoading(true)
    try {
      const res = await api.requests.list(tab === 'all' ? undefined : tab)
      setRequests(res.requests ?? [])
    } catch (e) {
      console.error('Failed to load requests', e)
    } finally {
      setLoading(false)
    }
  }

  async function openCredential(req: AccessRequest) {
    setCredentialFor(req)
    setCredential(null)
    setCredentialError('')
    setCredentialLoading(true)
    try {
      const c = await api.credentials.get(req.id)
      setCredential({ value: c.value ?? '', status: c.status ?? 'active', expires_at: (c as { expires_at?: string }).expires_at })
    } catch (e) {
      setCredentialError(e instanceof Error ? e.message : 'Failed to load credential')
    } finally {
      setCredentialLoading(false)
    }
  }

  async function revokeCredential() {
    if (!credentialFor) return
    try {
      await api.credentials.revoke(credentialFor.id)
      setNotice(`Credential revoked — the lease/credential is dead immediately.`)
      setCredentialFor(null)
      load()
    } catch (e) {
      setCredentialError(e instanceof Error ? e.message : 'Failed to revoke')
    }
  }

  useEffect(() => { load() }, [tab]) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Access Requests</h2>
        <p className="text-sm text-muted-foreground">Manage human-in-the-loop approvals</p>
      </div>

      {notice && (
        <div className="flex items-center justify-between rounded-md border border-green-300 bg-green-50 px-3 py-2 text-sm text-green-800 dark:border-green-900 dark:bg-green-950/30 dark:text-green-300">
          <span>✓ {notice}</span>
          <Button variant="ghost" size="sm" onClick={() => setNotice('')}>Dismiss</Button>
        </div>
      )}

      <div className="flex gap-1 border-b">
        {TABS.map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm font-medium capitalize border-b-2 transition-colors ${
              tab === t ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="text-sm text-muted-foreground py-8 text-center">Loading…</div>
      ) : requests.length === 0 ? (
        <div className="text-sm text-muted-foreground py-8 text-center">No requests found.</div>
      ) : (
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Secret</TableHead>
                <TableHead>Reason</TableHead>
                <TableHead>Duration</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-24">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {requests.map((req) => (
                <TableRow key={req.id}>
                  <TableCell className="text-sm font-medium">
                    {req.secret_name ?? `${req.secret_id.slice(0, 8)}…`}
                  </TableCell>
                  <TableCell className="max-w-[200px] truncate text-sm">{req.reason}</TableCell>
                  <TableCell className="text-sm">
                    {req.requested_duration_minutes >= 60
                      ? `${Math.floor(req.requested_duration_minutes / 60)} hr${Math.floor(req.requested_duration_minutes / 60) > 1 ? 's' : ''}`
                      : `${req.requested_duration_minutes} min`}
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(req.status)}>{req.status}</Badge>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">{formatDate(req.created_at)}</TableCell>
                  <TableCell>
                    {req.status === 'pending' && (
                      <Button variant="outline" size="sm" onClick={() => setActionsFor(req)}>
                        Review
                      </Button>
                    )}
                    {req.status === 'approved' && (
                      <Button variant="outline" size="sm" onClick={() => openCredential(req)}>
                        View credential
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <ApprovalActions
        request={actionsFor}
        onClose={() => setActionsFor(null)}
        onSuccess={() => { setActionsFor(null); load() }}
        onResult={(msg) => setNotice(msg)}
      />

      <Dialog open={!!credentialFor} onOpenChange={(v) => !v && setCredentialFor(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Granted credential</DialogTitle>
            <DialogDescription>
              {credentialFor?.secret_name ?? credentialFor?.secret_id.slice(0, 8)} — issued for{" "}
              {credentialFor?.requested_duration_minutes} minutes.
            </DialogDescription>
          </DialogHeader>
          {credentialLoading ? (
            <div className="py-4 text-sm text-muted-foreground">Loading…</div>
          ) : credentialError ? (
            <p className="text-sm text-destructive">{credentialError}</p>
          ) : credential ? (
            <div className="space-y-3">
              <div className="rounded-md bg-muted p-3 font-mono text-xs break-all">{credential.value}</div>
              <div className="text-sm text-muted-foreground">
                Status: {credential.status}
                {credential.expires_at ? ` · expires ${formatDate(credential.expires_at)}` : ''}
              </div>
              <DialogFooter className="gap-2">
                <Button variant="outline" onClick={() => setCredentialFor(null)}>Close</Button>
                <Button variant="destructive" onClick={revokeCredential}>Revoke now</Button>
              </DialogFooter>
            </div>
          ) : null}
        </DialogContent>
      </Dialog>
    </div>
  )
}
