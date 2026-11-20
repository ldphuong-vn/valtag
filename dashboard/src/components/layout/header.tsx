import { getSession } from '@/lib/auth'
import HeaderBell from './header-bell'

const routeLabels: Record<string, string> = {
  '/overview': 'Overview',
  '/secrets': 'Secrets',
  '/approvals': 'Approvals',
  '/audit': 'Audit Logs',
  '/settings': 'Settings',
}

interface HeaderProps {
  pathname?: string
}

export default async function Header({ pathname = '' }: HeaderProps) {
  const session = await getSession()
  const label = Object.entries(routeLabels).find(([k]) => pathname.startsWith(k))?.[1] ?? 'Dashboard'

  return (
    <header className="flex items-center justify-between h-14 px-6 border-b bg-card">
      <h1 className="text-sm font-semibold">{label}</h1>
      <div className="flex items-center gap-3">
        <HeaderBell />
        <div className="w-7 h-7 rounded-full bg-primary/20 flex items-center justify-center text-xs font-medium text-primary">
          {session?.email?.[0]?.toUpperCase() ?? 'U'}
        </div>
        <span className="text-xs text-muted-foreground hidden sm:block">{session?.email}</span>
      </div>
    </header>
  )
}
