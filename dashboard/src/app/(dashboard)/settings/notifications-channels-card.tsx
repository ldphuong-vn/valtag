'use client'

import { useState, useEffect } from 'react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Trash2 } from 'lucide-react'
import { api } from '@/lib/api-client'
import type { NotificationChannel } from '@/types/api'

const CHANNEL_LABELS: Record<string, string> = {
  email: 'Email',
  slack: 'Slack User ID',
  telegram: 'Telegram Chat ID',
}

interface SlackIntegration {
  id: string
  provider: string
  team_name: string
  workspace_id: string
}

// Where approval requests reach this user outside the dashboard. Embedded
// directly in /settings (2026-11-21) — the separate Notifications page was
// folded in so all personal settings live on one page. The Slack OAuth
// callback now lands back on /settings?slack=connected.
export default function NotificationsChannelsCard() {
  const [channels, setChannels] = useState<NotificationChannel[]>([])
  const [channelType, setChannelType] = useState('email')
  const [handle, setHandle] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [slackIntegration, setSlackIntegration] = useState<SlackIntegration | null>(null)
  const [slackConnected, setSlackConnected] = useState(false)

  useEffect(() => {
    api.notificationChannels.list().then(res => setChannels(res.channels)).catch(() => {})
    // Load Slack workspace integration
    api.orgs.list().then(res => {
      if (res.orgs.length > 0) {
        api.integrations.list(res.orgs[0].id).then(intRes => {
          const slack = intRes.integrations.find(i => i.provider === 'slack')
          if (slack) setSlackIntegration(slack as SlackIntegration)
        }).catch(() => {})
      }
    }).catch(() => {})
    // Check for connected query param
    const params = new URLSearchParams(window.location.search)
    if (params.get('slack') === 'connected') setSlackConnected(true)
  }, [])

  async function handleAdd() {
    if (!handle.trim()) return
    setLoading(true)
    setError('')
    try {
      const ch = await api.notificationChannels.upsert(channelType, handle.trim())
      setChannels(prev => {
        const filtered = prev.filter(c => c.channel_type !== ch.channel_type)
        return [...filtered, ch]
      })
      setHandle('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save')
    } finally {
      setLoading(false)
    }
  }

  async function handleTelegramConnect() {
    setLoading(true)
    setError('')
    try {
      const { url } = await api.notificationChannels.telegramLink()
      window.open(url, '_blank', 'noopener,noreferrer')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to generate Telegram link')
    } finally {
      setLoading(false)
    }
  }

  async function handleDelete(id: string) {
    try {
      await api.notificationChannels.delete(id)
      setChannels(prev => prev.filter(c => c.id !== id))
    } catch {
      setError('Failed to remove channel')
    }
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Add channel</CardTitle>
          <CardDescription>
            Link channels to receive approval notifications. One channel per type;
            adding again replaces the existing one. Slack and Telegram require
            verification after linking.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex gap-2">
            <Select value={channelType} onValueChange={v => { setChannelType(v); setHandle('') }}>
              <SelectTrigger className="w-36">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="email">Email</SelectItem>
                <SelectItem value="slack">Slack</SelectItem>
                <SelectItem value="telegram">Telegram</SelectItem>
              </SelectContent>
            </Select>
            {channelType === 'telegram' ? (
              <Button onClick={handleTelegramConnect} disabled={loading} className="flex-1">
                {loading ? 'Opening…' : 'Connect via Telegram →'}
              </Button>
            ) : (
              <>
                <Input
                  placeholder={CHANNEL_LABELS[channelType]}
                  value={handle}
                  onChange={e => setHandle(e.target.value)}
                  onKeyDown={e => e.key === 'Enter' && handleAdd()}
                  className="flex-1"
                />
                <Button onClick={handleAdd} disabled={loading || !handle.trim()}>
                  {loading ? 'Saving…' : 'Save'}
                </Button>
              </>
            )}
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
        </CardContent>
      </Card>

      {channels.length > 0 && (
        <Card>
          <CardHeader><CardTitle className="text-base">Linked channels</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {channels.map(ch => (
              <div key={ch.id} className="flex items-center justify-between rounded-md border px-3 py-2">
                <div className="space-y-0.5">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium capitalize">{ch.channel_type}</span>
                    {ch.verified
                      ? <Badge variant="default" className="text-xs">Verified</Badge>
                      : <Badge variant="secondary" className="text-xs">Pending verification</Badge>
                    }
                  </div>
                  <p className="text-xs text-muted-foreground font-mono">{ch.handle}</p>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 text-muted-foreground hover:text-destructive"
                  onClick={() => handleDelete(ch.id)}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {/* Slack Workspace Integration (org-level) */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Slack workspace</CardTitle>
          <CardDescription>Connect your org&apos;s Slack workspace to receive approval notifications.</CardDescription>
        </CardHeader>
        <CardContent>
          {slackConnected && !slackIntegration && (
            <p className="text-sm text-green-600 mb-2">Slack connected successfully! Refresh to see details.</p>
          )}
          {slackIntegration ? (
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">Connected</span>
                  <Badge variant="default" className="text-xs">{slackIntegration.team_name}</Badge>
                </div>
                <p className="text-xs text-muted-foreground">Workspace ID: {slackIntegration.workspace_id}</p>
              </div>
              <Button
                variant="outline"
                size="sm"
                className="text-destructive"
                onClick={async () => {
                  const orgs = await api.orgs.list()
                  if (orgs.orgs.length > 0) {
                    await api.integrations.disconnectSlack(orgs.orgs[0].id)
                    setSlackIntegration(null)
                  }
                }}
              >
                Disconnect
              </Button>
            </div>
          ) : (
            <Button
              variant="outline"
              onClick={async () => {
                const orgs = await api.orgs.list()
                if (orgs.orgs.length > 0) {
                  window.location.href = `/api/proxy/oauth/slack?org_id=${orgs.orgs[0].id}`
                }
              }}
            >
              Connect Slack Workspace
            </Button>
          )}
        </CardContent>
      </Card>
    </>
  )
}
