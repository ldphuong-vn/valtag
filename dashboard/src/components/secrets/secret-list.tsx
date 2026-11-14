'use client'

import { useEffect, useRef, useState } from 'react'
import { Plus, Pencil, Trash2, KeyRound, Shield, ShieldAlert, ClipboardCheck } from 'lucide-react'
import { ApiError, api } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import type { Secret } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import SecretForm from './secret-form'
import { SecretPolicyBindingDialog } from './secret-policy-binding-dialog'

export default function SecretList() {
  const [secrets, setSecrets] = useState<Secret[]>([])
  const [projectId, setProjectId] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Secret | null>(null)

  const [policyDialogOpen, setPolicyDialogOpen] = useState(false)
  const [policySecretId, setPolicySecretId] = useState<string | null>(null)
  const [createdSecretForPolicyId, setCreatedSecretForPolicyId] = useState<string | null>(null)
  const [policyBoundBySecretId, setPolicyBoundBySecretId] = useState<Record<string, boolean | null>>({})
  // AX (dogfood 2026-11-12): the ask-first flow had no human entry point —
  // request access lives here now.
  const [accessDialogOpen, setAccessDialogOpen] = useState(false)
  const [accessSecret, setAccessSecret] = useState<Secret | null>(null)
  const [accessReason, setAccessReason] = useState('')
  const [accessDuration, setAccessDuration] = useState(30)
  const [accessSubmitting, setAccessSubmitting] = useState(false)
  const [accessResult, setAccessResult] = useState<{ status: string; request_id: string } | null>(null)
  const [credentialValue, setCredentialValue] = useState('')
  const [copied, setCopied] = useState(false)
  const [accessError, setAccessError] = useState('')
  const loadSeqRef = useRef(0)

  async function loadBindingIndicators(list: Secret[], loadSeq: number) {
    if (list.length === 0) {
      setPolicyBoundBySecretId({})
      return
    }
    setPolicyBoundBySecretId(Object.fromEntries(list.map((secret) => [secret.id, null])))
    await Promise.all(
      list.map(async (secret) => {
        let isBound: boolean | null = null
        try {
          const binding = await api.policies.getBinding(secret.id)
          isBound = Boolean(binding.template?.id)
        } catch (err) {
          if (err instanceof ApiError && err.status === 404) isBound = false
        }
        if (loadSeqRef.current !== loadSeq || isBound === null) return
        setPolicyBoundBySecretId((prev) => {
          if (!(secret.id in prev)) return prev
          return { ...prev, [secret.id]: isBound }
        })
      })
    )
  }

  async function load() {
    const loadSeq = ++loadSeqRef.current
    try {
      const res = await api.secrets.list()
      const list = res.secrets ?? []
      if (loadSeqRef.current !== loadSeq) return
      setSecrets(list)
      void loadBindingIndicators(list, loadSeq)
    } catch (e) {
      if (loadSeqRef.current !== loadSeq) return
      console.error('Failed to load secrets', e)
    } finally {
      if (loadSeqRef.current !== loadSeq) return
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  useEffect(() => {
    const pid = typeof window !== 'undefined' ? localStorage.getItem('valt_current_project') : null
    setProjectId(pid)
  }, [])

  async function handleDelete(id: string) {
    if (!confirm('Delete this secret?')) return
    await api.secrets.delete(id)
    load()
  }

  function openPolicyDialog(secretId: string) {
    setPolicySecretId(secretId)
    setPolicyDialogOpen(true)
  }

  function openAccessDialog(s: Secret) {
    setAccessSecret(s)
    setAccessReason('')
    setAccessDuration(30)
    setAccessResult(null)
    setCredentialValue('')
    setAccessError('')
    setAccessDialogOpen(true)
  }

  async function handleAccessSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!accessSecret || !accessReason.trim()) return
    setAccessSubmitting(true)
    setAccessError('')
    try {
      const req = await api.requests.create(accessSecret.id, {
        reason: accessReason.trim(),
        duration_minutes: accessDuration,
      })
      const status = (req.status ?? 'pending') as string
      setAccessResult({ status, request_id: req.id })
      // Auto-approved → fetch the value right away so the dialog doubles as
      // "reveal + copy" for human-held credentials (dogfood 2026-11-13).
      if (status === 'approved') {
        const cred = await api.credentials.get(req.id)
        setCredentialValue(cred.value ?? '')
      }
    } catch (err) {
      setAccessError(err instanceof Error ? err.message : 'Failed to create access request')
    } finally {
      setAccessSubmitting(false)
    }
  }

  async function copyValue(text: string) {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      // clipboard denied — the value stays selectable in the dialog anyway
    }
  }

  const policySecretName = secrets.find((s) => s.id === policySecretId)?.name

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Secrets</h2>
          <p className="text-sm text-muted-foreground">{secrets.length} secrets stored</p>
        </div>
        <Button onClick={() => { setEditing(null); setFormOpen(true) }}>
          <Plus className="w-4 h-4 mr-2" /> New Secret
        </Button>
      </div>

      {createdSecretForPolicyId && (
        <div className="flex items-center justify-between rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm dark:bg-amber-950/30">
          <div className="flex items-center gap-2 text-amber-900 dark:text-amber-300">
            <ShieldAlert className="w-4 h-4" />
            <p>Secret created. Apply a policy now.</p>
          </div>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={() => {
                openPolicyDialog(createdSecretForPolicyId)
                setCreatedSecretForPolicyId(null)
              }}
            >
              <Shield className="w-3.5 h-3.5 mr-1" /> Apply policy now
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setCreatedSecretForPolicyId(null)}>
              Later
            </Button>
          </div>
        </div>
      )}

      {loading ? (
        <div className="text-sm text-muted-foreground py-8 text-center">Loading…</div>
      ) : secrets.length === 0 ? (
        <div className="flex flex-col items-center py-16 gap-3 text-muted-foreground">
          <KeyRound className="w-10 h-10 opacity-30" />
          <p className="text-sm">No secrets yet. Create your first one.</p>
        </div>
      ) : (
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-32">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {secrets.map((s) => (
                <TableRow key={s.id}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{s.name}</span>
                      {policyBoundBySecretId[s.id] === false && (
                        <Badge
                          variant="outline"
                          title="Secret has no access policy bound."
                          className="gap-1 border-amber-300 text-amber-700"
                        >
                          <ShieldAlert className="w-3 h-3" />
                          No policy
                        </Badge>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary">{s.credential_type}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={s.status === 'active' ? 'default' : 'outline'}>{s.status}</Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground text-xs">{formatDate(s.created_at)}</TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <Button variant="ghost" size="sm" title="Request access (ask-first flow)" onClick={() => openAccessDialog(s)}>
                        <ClipboardCheck className="w-3.5 h-3.5 mr-1" /> Request access
                      </Button>
                      <Button variant="ghost" size="icon" title="Edit Secret" onClick={() => { setEditing(s); setFormOpen(true) }}>
                        <Pencil className="w-3.5 h-3.5" />
                      </Button>
                      <Button variant="ghost" size="sm" title="Manage Policies" onClick={() => openPolicyDialog(s.id)}>
                        <Shield className="w-3.5 h-3.5 mr-1" /> Manage Policies
                      </Button>
                      <Button variant="ghost" size="icon" title="Delete Secret" onClick={() => handleDelete(s.id)}>
                        <Trash2 className="w-3.5 h-3.5 text-destructive" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <SecretForm
        open={formOpen}
        secret={editing}
        projectId={projectId}
        onClose={() => setFormOpen(false)}
        onSuccess={() => { setFormOpen(false); load() }}
        onCreated={(id) => setCreatedSecretForPolicyId(id)}
      />

      <SecretPolicyBindingDialog
        open={policyDialogOpen}
        secretId={policySecretId}
        secretName={policySecretName}
        onClose={() => {
          setPolicyDialogOpen(false)
          setPolicySecretId(null)
        }}
        onSaved={() => {
          setPolicyDialogOpen(false)
          setPolicySecretId(null)
          load()
        }}
      />

      <Dialog open={accessDialogOpen} onOpenChange={(o) => { if (!o) { setAccessDialogOpen(false); setAccessSecret(null) } }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Request access — {accessSecret?.name}</DialogTitle>
            <DialogDescription>
              Ask-first: an approver is notified. If no policy binds this secret, the request is auto-approved.
            </DialogDescription>
          </DialogHeader>
          {accessResult ? (
            <div className="space-y-3 py-2 text-sm">
              {accessResult.status === 'approved' ? (
                credentialValue ? (
                  <>
                    <p className="text-green-700 dark:text-green-400">✓ Approved — credential below. It expires with the lease; re-request anytime.</p>
                    <div className="rounded-md bg-muted p-3 font-mono text-xs whitespace-pre-wrap break-all">{credentialValue}</div>
                    <Button size="sm" variant="outline" onClick={() => copyValue(credentialValue)}>
                      {copied ? '✓ Copied' : 'Copy all'}
                    </Button>
                  </>
                ) : (
                  <p className="text-green-700 dark:text-green-400">✓ Approved. Retrieve with <code className="text-xs">valtag get {accessSecret?.name}</code> or an agent request.</p>
                )
              ) : (
                <p className="text-amber-700 dark:text-amber-400">
                  ⏳ Request submitted ({accessResult.request_id.slice(0, 8)}…) — track it on the Approvals page. The approver has been notified.
                </p>
              )}
              <DialogFooter>
                <Button variant="outline" onClick={() => { setAccessDialogOpen(false); setAccessSecret(null) }}>
                  Close
                </Button>
              </DialogFooter>
            </div>
          ) : (
            <form onSubmit={handleAccessSubmit} className="space-y-3 py-1">
              <div>
                <label className="text-sm font-medium">Reason *</label>
                <Textarea
                  value={accessReason}
                  onChange={(e) => setAccessReason(e.target.value)}
                  placeholder="What is this access for? The approver reads this."
                  rows={3}
                  required
                />
              </div>
              <div>
                <label className="text-sm font-medium">Duration</label>
                <select
                  className="w-full rounded-md border bg-transparent px-3 py-2 text-sm"
                  value={accessDuration}
                  onChange={(e) => setAccessDuration(Number(e.target.value))}
                >
                  <option value={15}>15 minutes</option>
                  <option value={30}>30 minutes</option>
                  <option value={60}>1 hour</option>
                </select>
              </div>
              {accessError && <p className="text-sm text-destructive">{accessError}</p>}
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => { setAccessDialogOpen(false); setAccessSecret(null) }}>
                  Cancel
                </Button>
                <Button type="submit" disabled={accessSubmitting || !accessReason.trim()}>
                  {accessSubmitting ? 'Submitting…' : 'Submit request'}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
