'use client'

import { useState } from 'react'
import { api } from '@/lib/api-client'
import type { AccessRequest } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'

interface Props {
  request: AccessRequest | null
  onClose: () => void
  onSuccess: () => void
  onResult?: (message: string) => void
}

export default function ApprovalActions({ request, onClose, onSuccess, onResult }: Props) {
  const [reason, setReason] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function handleApprove() {
    if (!request) return
    setLoading(true)
    setError('')
    try {
      await api.requests.approve(request.id, { reason: reason || undefined })
      onResult?.(`Approved "${request.secret_name ?? request.id.slice(0, 8)}…" — the requester can now retrieve the credential.`)
      onSuccess()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to approve')
    } finally {
      setLoading(false)
    }
  }

  async function handleReject() {
    if (!request) return
    if (!reason.trim()) { setError('Rejection reason is required'); return }
    setLoading(true)
    setError('')
    try {
      await api.requests.reject(request.id, { reason })
      onResult?.(`Rejected "${request.secret_name ?? request.id.slice(0, 8)}…" — the requester sees the reason on their next check.`)
      onSuccess()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to reject')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={!!request} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Review Access Request</DialogTitle>
        </DialogHeader>
        {request && (
          <div className="space-y-4 mt-2">
            <div className="rounded-md bg-muted p-3 text-sm space-y-1">
              <div><span className="text-muted-foreground">Reason: </span>{request.reason}</div>
              <div><span className="text-muted-foreground">Duration: </span>{request.requested_duration_minutes} minutes</div>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="review-reason">Note (required for rejection)</Label>
              <Textarea
                id="review-reason"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="Optional for approval, required for rejection…"
                rows={3}
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <DialogFooter className="gap-2">
              <Button variant="outline" onClick={onClose} disabled={loading}>Cancel</Button>
              <Button variant="destructive" onClick={handleReject} disabled={loading}>Reject</Button>
              <Button onClick={handleApprove} disabled={loading}>Approve</Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
