'use client'

import { useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export default function ChangePasswordCard() {
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    if (newPassword !== confirm) {
      setError('New password and confirmation do not match')
      return
    }
    setLoading(true)
    try {
      const res = await fetch('/api/proxy/auth/change-password', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        setError((data as { detail?: string; message?: string; error?: string }).detail
          ?? (data as { error?: { message?: string } }).error?.message
          ?? 'Failed to change password')
        return
      }
      setDone(true)
      // Password change revokes all sessions server-side (refresh tokens
      // deleted + token_version bumped). Sign this session out immediately —
      // matching the Higress console behaviour the operator expects.
      setTimeout(async () => {
        await fetch('/api/auth/logout', { method: 'POST', credentials: 'include' })
        window.location.href = '/login'
      }, 1200)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Card>
      <CardHeader><CardTitle className="text-base">Change password</CardTitle></CardHeader>
      <CardContent>
        {done ? (
          <div className="space-y-2 text-sm">
            <p className="text-green-700 dark:text-green-400">✓ Password updated.</p>
            <p className="text-muted-foreground">All sessions have been signed out. Sign in with your new password.</p>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="current-password">Current password</Label>
              <Input
                id="current-password"
                type="password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                required
                autoComplete="current-password"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="new-password">New password</Label>
              <Input
                id="new-password"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                required
                minLength={12}
                autoComplete="new-password"
              />
              <p className="text-xs text-muted-foreground">At least 12 characters, with upper, lower and a digit.</p>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="confirm-password">Confirm new password</Label>
              <Input
                id="confirm-password"
                type="password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                required
                autoComplete="new-password"
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button type="submit" disabled={loading}>
              {loading ? 'Updating…' : 'Update password'}
            </Button>
          </form>
        )}
      </CardContent>
    </Card>
  )
}
