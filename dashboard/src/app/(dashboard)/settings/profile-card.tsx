'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

interface Me {
  id: string
  email: string
  role: string
  email_verified?: boolean
  created_at?: string
  display_name?: string
}

const MAX_NAME = 100

export default function ProfileCard({ fallbackId }: { fallbackId: string }) {
  const [me, setMe] = useState<Me | null>(null)
  const [failed, setFailed] = useState(false)
  const [editing, setEditing] = useState(false)
  const [draftName, setDraftName] = useState('')
  const [saving, setSaving] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')

  async function load() {
    try {
      const r = await fetch('/api/proxy/auth/me', { credentials: 'include' })
      if (!r.ok) throw new Error(String(r.status))
      const data: Me = await r.json()
      setMe(data)
      setDraftName(data.display_name ?? '')
    } catch {
      setFailed(true)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  async function saveName() {
    setSaving(true)
    setError('')
    setNotice('')
    try {
      const r = await fetch('/api/proxy/me/profile', {
        method: 'PATCH',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ display_name: draftName }),
      })
      if (!r.ok) {
        const body = await r.text()
        throw new Error(body.includes('too long') ? 'Too long (max 100 characters)' : `Failed (${r.status})`)
      }
      const data = await r.json()
      setMe(m => (m ? { ...m, display_name: data.display_name ?? '' } : m))
      setNotice('Saved')
      setEditing(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  const shownName = me?.display_name?.trim() || ''
  const initials = (shownName || me?.email || fallbackId || 'U').replace(/^sk-/, '').slice(0, 1).toUpperCase()
  const memberSince = me?.created_at ? new Date(me.created_at).toLocaleDateString() : null

  return (
    <Card>
      <CardHeader><CardTitle className="text-base">Profile</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center gap-4">
          <Avatar className="w-12 h-12">
            <AvatarFallback>{initials}</AvatarFallback>
          </Avatar>
          <div className="min-w-0">
            {failed ? (
              <p className="text-sm text-muted-foreground">Profile unavailable (session expired?)</p>
            ) : me ? (
              <>
                <p className="font-medium text-sm truncate">{shownName || me.email}</p>
                <div className="flex items-center gap-2 mt-1">
                  <Badge variant="secondary">{me.role}</Badge>
                  {me.email_verified === true && (
                    <Badge variant="secondary" className="text-green-600 dark:text-green-400 border border-green-600/30 bg-green-500/10">
                      ✓ email verified
                    </Badge>
                  )}
                </div>
              </>
            ) : (
              <p className="text-sm text-muted-foreground">Loading…</p>
            )}
          </div>
        </div>

        {me && !failed && (
          <div className="space-y-3 border-t pt-4">
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Display name</span>
                {!editing && (
                  <Button size="sm" variant="ghost" onClick={() => { setEditing(true); setNotice(''); setError('') }}>
                    ✎ Edit
                  </Button>
                )}
              </div>
              {editing ? (
                <div className="space-y-2">
                  <Input
                    value={draftName}
                    maxLength={MAX_NAME}
                    placeholder="How your name appears in approvals and audit context"
                    onChange={(e) => setDraftName(e.target.value)}
                    onKeyDown={(e) => { if (e.key === 'Enter') void saveName() }}
                  />
                  <div className="flex items-center gap-2">
                    <Button size="sm" disabled={saving} onClick={() => void saveName()}>
                      {saving ? 'Saving…' : 'Save'}
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => { setEditing(false); setDraftName(me.display_name ?? ''); setError('') }}>
                      Cancel
                    </Button>
                    {notice && <span className="text-xs text-green-600">{notice}</span>}
                    {error && <span className="text-xs text-destructive">{error}</span>}
                  </div>
                </div>
              ) : (
                <p className="text-sm">{shownName || <span className="text-muted-foreground italic">Not set — add the name your approvals should show</span>}</p>
              )}
            </div>

            <div className="space-y-2 text-sm">
              <div className="flex justify-between items-center">
                <span className="text-muted-foreground">Email (account)</span>
                <span className="font-mono text-xs truncate max-w-[240px]">{me.email}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-muted-foreground">User ID</span>
                <span className="font-mono text-xs">{me.id.slice(0, 16)}…</span>
              </div>
              {memberSince && (
                <div className="flex justify-between items-center">
                  <span className="text-muted-foreground">Member since</span>
                  <span className="text-xs">{memberSince}</span>
                </div>
              )}
            </div>
            <p className="text-xs text-muted-foreground">
              Changing the account email is a separate verified flow — not editable here.
            </p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
