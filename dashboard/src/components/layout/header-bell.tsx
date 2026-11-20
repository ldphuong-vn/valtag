'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { Bell } from 'lucide-react'

// Topbar bell: live count of approvals waiting for this user. Polls once a
// minute; clicking goes straight to Approvals.
export default function HeaderBell() {
  const [count, setCount] = useState<number | null>(null)

  useEffect(() => {
    let alive = true
    async function tick() {
      try {
        const r = await fetch('/api/proxy/access-requests?status=pending', { credentials: 'include' })
        if (!r.ok) return
        const d = await r.json()
        if (alive) setCount((d.requests ?? []).length)
      } catch { /* stay quiet — a badge is optional */ }
    }
    void tick()
    const t = setInterval(tick, 60_000)
    return () => { alive = false; clearInterval(t) }
  }, [])

  return (
    <Link
      href="/approvals"
      title="Pending approvals"
      className="relative flex items-center justify-center w-8 h-8 rounded-full hover:bg-accent transition-colors"
    >
      <Bell className="w-4 h-4 text-muted-foreground" />
      {count !== null && count > 0 && (
        <span className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 px-1 rounded-full bg-destructive text-destructive-foreground text-[10px] font-bold grid place-items-center">
          {count}
        </span>
      )}
    </Link>
  )
}
