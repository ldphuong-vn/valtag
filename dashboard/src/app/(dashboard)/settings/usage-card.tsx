'use client'

import { useEffect, useState } from 'react'
import { api } from '@/lib/api-client'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const PLAN_LABELS: Record<string, string> = {
  free: 'Free',
  pro: 'Pro',
  team: 'Team',
}

const PLAN_DESCRIPTIONS: Record<string, string> = {
  free: 'Free tier includes 50 secrets, 3 agents, and 1,000 API requests per day.',
  pro: 'Pro plan includes 500 secrets, 25 agents, and 10,000 API requests per day.',
  team: 'Team plan includes unlimited secrets, unlimited agents, and 50,000 API requests per day.',
}

function UsageBar({ label, current, limit }: { label: string; current: number; limit: number }) {
  const isUnlimited = limit < 0
  const pct = isUnlimited ? 0 : Math.min((current / limit) * 100, 100)
  return (
    <div className="space-y-1">
      <div className="flex justify-between text-sm">
        <span>{label}</span>
        <span className="text-muted-foreground">
          {current} / {isUnlimited ? '∞' : limit.toLocaleString()}
        </span>
      </div>
      <div className="h-1.5 rounded-full bg-muted overflow-hidden">
        <div
          className="h-full bg-primary rounded-full transition-all"
          style={{ width: isUnlimited ? '0%' : `${pct}%` }}
        />
      </div>
    </div>
  )
}

export default function UsageCard() {
  const [data, setData] = useState<{
    plan: string
    usage: Record<string, number>
    limits: Record<string, number>
  } | null>(null)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    async function load() {
      try {
        const orgs = await api.orgs.list()
        if (orgs.orgs.length > 0) {
          const usage = await api.usage.get(orgs.orgs[0].id)
          setData(usage)
        } else {
          setFailed(true)
        }
      } catch {
        setFailed(true)
      }
    }
    load()
  }, [])

  if (failed) return null
  if (!data) {
    return (
      <Card>
        <CardHeader><CardTitle className="text-base">Plan &amp; Usage</CardTitle></CardHeader>
        <CardContent><p className="text-sm text-muted-foreground">Loading usage…</p></CardContent>
      </Card>
    )
  }

  const plan = data?.plan ?? 'free'
  const usage = data?.usage ?? { secrets_count: 0, agents_count: 0, requests_today: 0 }
  const limits = data?.limits ?? { secrets_count: 50, agents_count: 3, requests_today: 1000 }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base flex items-center justify-between">
          Plan &amp; Usage
          <span className="px-2 py-0.5 rounded-full text-xs bg-secondary text-secondary-foreground font-normal">
            {PLAN_LABELS[plan] ?? plan}
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-sm text-muted-foreground">
          {PLAN_DESCRIPTIONS[plan] ?? 'Custom plan.'}
        </p>
        <UsageBar label="Secrets" current={usage.secrets_count} limit={limits.secrets_count} />
        <UsageBar label="Agents" current={usage.agents_count} limit={limits.agents_count} />
        <UsageBar label="API Requests (today)" current={usage.requests_today} limit={limits.requests_today} />
      </CardContent>
    </Card>
  )
}
