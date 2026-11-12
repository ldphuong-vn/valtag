'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'

interface Me {
  id: string
  email: string
  role: string
}

export default function ProfileCard({ fallbackId }: { fallbackId: string }) {
  const [me, setMe] = useState<Me | null>(null)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    fetch('/api/proxy/auth/me', { credentials: 'include' })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(String(r.status)))))
      .then(setMe)
      .catch(() => setFailed(true))
  }, [])

  const email = me?.email ?? ''
  return (
    <Card>
      <CardHeader><CardTitle className="text-base">Profile</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center gap-4">
          <Avatar className="w-12 h-12">
            <AvatarFallback>{email?.[0]?.toUpperCase() ?? fallbackId?.[0]?.toUpperCase() ?? 'U'}</AvatarFallback>
          </Avatar>
          <div>
            {failed ? (
              <p className="text-sm text-muted-foreground">Profile unavailable (session expired?)</p>
            ) : me ? (
              <>
                <p className="font-medium text-sm">{me.email}</p>
                <Badge variant="secondary" className="mt-1">{me.role}</Badge>
              </>
            ) : (
              <p className="text-sm text-muted-foreground">Loading…</p>
            )}
          </div>
        </div>

        <div className="text-sm space-y-2">
          <div className="flex justify-between">
            <span className="text-muted-foreground">User ID</span>
            <span className="font-mono text-xs">{(me?.id ?? fallbackId).slice(0, 16)}…</span>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
