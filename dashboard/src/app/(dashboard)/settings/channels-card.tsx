'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

interface NotificationChannel {
  id: string
  channel_type: string
  handle: string
  verified: boolean
}

const CHANNEL_LABELS: Record<string, string> = {
  telegram: 'Telegram',
  zalo: 'Zalo',
  slack: 'Slack workspace',
}

// Where approval requests reach this user outside the dashboard. Channels are
// configured on the Notifications page (bot linking etc.) — this card mirrors
// their status like 1Password's contact-channels section.
export default function ChannelsCard() {
  const [channels, setChannels] = useState<NotificationChannel[] | null>(null)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    fetch('/api/proxy/me/notification-channels', { credentials: 'include' })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(String(r.status)))))
      .then((d) => setChannels(d.channels ?? []))
      .catch(() => setFailed(true))
  }, [])

  const byType = new Map<string, NotificationChannel>()
  for (const c of channels ?? []) byType.set(c.channel_type, c)
  const entries = Object.entries(CHANNEL_LABELS).map(([type, label]) => ({
    type,
    label,
    channel: byType.get(type) ?? null,
  }))

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Contact channels</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">
          Where approval requests reach you when you&apos;re not in the dashboard.
        </p>

        {failed ? (
          <p className="text-sm text-muted-foreground">Channel status unavailable.</p>
        ) : channels === null ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : (
          <div className="space-y-2">
            {entries.map(({ type, label, channel }) => (
              <div key={type} className="flex items-center justify-between rounded-md border px-3 py-2">
                <div className="flex items-center gap-2.5 min-w-0">
                  <span className="text-base">
                    {type === 'telegram' ? '✈️' : type === 'zalo' ? '💬' : type === 'slack' ? '💼' : '📨'}
                  </span>
                  <div className="min-w-0">
                    <div className="text-sm font-medium">{label}</div>
                    <div className="text-xs text-muted-foreground truncate font-mono">
                      {channel?.handle ?? 'not connected'}
                    </div>
                  </div>
                </div>
                {channel ? (
                  channel.verified ? (
                    <Badge variant="secondary" className="text-green-600 dark:text-green-400 border border-green-600/30 bg-green-500/10">
                      connected
                    </Badge>
                  ) : (
                    <Badge variant="secondary" className="text-yellow-600 dark:text-yellow-400 border border-yellow-600/30 bg-yellow-500/10">
                      pending verify
                    </Badge>
                  )
                ) : (
                  <Badge variant="secondary">—</Badge>
                )}
              </div>
            ))}
            <div className="text-sm">
              Email: approval links go to your account email (see Profile).
            </div>
          </div>
        )}

        <Link
          href="/settings/notifications"
          className="inline-flex text-sm text-primary hover:underline"
        >
          Configure channels → Notifications
        </Link>
      </CardContent>
    </Card>
  )
}
