'use client'

import { useEffect, useState } from 'react'
import { api, ApiError } from '@/lib/api-client'
import { Shield, Users, Building2, KeyRound, Activity, UserPlus, CreditCard } from 'lucide-react'

// ---------------------------------------------------------------------------
// Types (inferred from api-client shapes)
// ---------------------------------------------------------------------------
type Stats = Awaited<ReturnType<typeof api.admin.stats>>
type UserRow = Awaited<ReturnType<typeof api.admin.users>>['users'][number]
type OrgRow = Awaited<ReturnType<typeof api.admin.orgs>>['orgs'][number]
type PaymentRow = Awaited<ReturnType<typeof api.admin.payments>>['payments'][number]

// ---------------------------------------------------------------------------
// Small reusable stat card
// ---------------------------------------------------------------------------
function StatCard({
  label,
  value,
  icon: Icon,
  sub,
}: {
  label: string
  value: number | string
  icon: React.ElementType
  sub?: string
}) {
  return (
    <div className="rounded-lg border bg-card p-5 flex gap-4 items-start">
      <div className="mt-0.5 rounded-md bg-primary/10 p-2">
        <Icon className="w-5 h-5 text-primary" />
      </div>
      <div>
        <p className="text-xs text-muted-foreground font-medium uppercase tracking-wide">{label}</p>
        <p className="text-2xl font-bold mt-0.5">{value}</p>
        {sub && <p className="text-xs text-muted-foreground mt-0.5">{sub}</p>}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Plan distribution bar
// ---------------------------------------------------------------------------
function PlanBar({ dist }: { dist: Record<string, number> }) {
  const plans = ['free', 'pro', 'team']
  const total = Object.values(dist).reduce((a, b) => a + b, 0) || 1
  const colors: Record<string, string> = {
    free: 'bg-slate-400',
    pro: 'bg-blue-500',
    team: 'bg-violet-500',
  }
  return (
    <div className="rounded-lg border bg-card p-5">
      <p className="text-sm font-semibold mb-4">Plan Distribution</p>
      <div className="space-y-3">
        {plans.map((plan) => {
          const count = dist[plan] ?? 0
          const pct = Math.round((count / total) * 100)
          return (
            <div key={plan} className="space-y-1">
              <div className="flex justify-between text-xs text-muted-foreground">
                <span className="capitalize font-medium">{plan}</span>
                <span>{count} orgs ({pct}%)</span>
              </div>
              <div className="h-2 rounded-full bg-muted overflow-hidden">
                <div
                  className={`h-full rounded-full ${colors[plan] ?? 'bg-primary'}`}
                  style={{ width: `${pct}%` }}
                />
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Table helpers
// ---------------------------------------------------------------------------
function TableHead({ cols }: { cols: string[] }) {
  return (
    <thead>
      <tr className="border-b bg-muted/40">
        {cols.map((c) => (
          <th key={c} className="px-3 py-2 text-left text-xs font-semibold text-muted-foreground uppercase tracking-wide whitespace-nowrap">
            {c}
          </th>
        ))}
      </tr>
    </thead>
  )
}

function Badge({ label, variant }: { label: string; variant?: 'green' | 'amber' | 'blue' | 'violet' | 'default' }) {
  const cls = {
    green: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
    amber: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
    blue: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
    violet: 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400',
    default: 'bg-muted text-muted-foreground',
  }[variant ?? 'default']
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}>
      {label}
    </span>
  )
}

function planVariant(plan: string): 'green' | 'blue' | 'violet' | 'default' {
  if (plan === 'pro') return 'blue'
  if (plan === 'team') return 'violet'
  return 'default'
}

function formatDate(iso: string) {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
  } catch {
    return iso
  }
}

// ---------------------------------------------------------------------------
// Main admin page
// ---------------------------------------------------------------------------
export default function AdminPage() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [users, setUsers] = useState<UserRow[]>([])
  const [orgs, setOrgs] = useState<OrgRow[]>([])
  const [payments, setPayments] = useState<PaymentRow[]>([])
  const [loading, setLoading] = useState(true)
  const [forbidden, setForbidden] = useState(false)
  const [search, setSearch] = useState('')
  const [searchInput, setSearchInput] = useState('')

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const [s, u, o, p] = await Promise.all([
          api.admin.stats(),
          api.admin.users({ search: search || undefined }),
          api.admin.orgs(),
          api.admin.payments(),
        ])
        if (cancelled) return
        setStats(s)
        setUsers(u.users)
        setOrgs(o.orgs)
        setPayments(p.payments)
      } catch (err) {
        if (cancelled) return
        if (err instanceof ApiError && err.status === 403) {
          setForbidden(true)
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => { cancelled = true }
  }, [search])

  if (forbidden) {
    return (
      <div className="flex flex-col items-center justify-center h-64 gap-3 text-muted-foreground">
        <Shield className="w-10 h-10" />
        <p className="text-lg font-semibold">Admin access required</p>
        <p className="text-sm">Your account does not have the admin role.</p>
      </div>
    )
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64 text-muted-foreground text-sm">
        Loading admin data...
      </div>
    )
  }

  return (
    <div className="space-y-8 max-w-6xl">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Shield className="w-6 h-6 text-primary" />
        <div>
          <h1 className="text-xl font-bold">Super Admin</h1>
          <p className="text-sm text-muted-foreground">System-wide monitoring for valtag operators</p>
        </div>
      </div>

      {/* Stat cards */}
      {stats && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <StatCard
            label="Total Users"
            value={stats.total_users}
            icon={Users}
            sub={`+${stats.users_today} today`}
          />
          <StatCard
            label="Total Orgs"
            value={stats.total_orgs}
            icon={Building2}
          />
          <StatCard
            label="Total Secrets"
            value={stats.total_secrets}
            icon={KeyRound}
          />
          <StatCard
            label="Requests Today"
            value={stats.total_requests_today}
            icon={Activity}
            sub={`${stats.total_requests_all} all time`}
          />
        </div>
      )}

      {/* Plan distribution */}
      {stats && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <PlanBar dist={stats.plan_distribution} />
          <StatCard
            label="Total Agents"
            value={stats.total_agents}
            icon={UserPlus}
          />
        </div>
      )}

      {/* Users table */}
      <section>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-base font-semibold">Users</h2>
          <form
            onSubmit={(e) => { e.preventDefault(); setSearch(searchInput) }}
            className="flex gap-2"
          >
            <input
              type="text"
              placeholder="Search by email..."
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              className="h-8 rounded-md border bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
            />
            <button
              type="submit"
              className="h-8 px-3 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
            >
              Search
            </button>
            {search && (
              <button
                type="button"
                onClick={() => { setSearch(''); setSearchInput('') }}
                className="h-8 px-3 rounded-md border text-sm hover:bg-muted transition-colors"
              >
                Clear
              </button>
            )}
          </form>
        </div>
        <div className="rounded-lg border overflow-hidden">
          <table className="w-full text-sm">
            <TableHead cols={['Email', 'Role', 'Status', 'Verified', '2FA', 'Joined']} />
            <tbody>
              {users.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-3 py-6 text-center text-muted-foreground text-sm">
                    No users found.
                  </td>
                </tr>
              )}
              {users.map((u) => (
                <tr key={u.id} className="border-b last:border-0 hover:bg-muted/30 transition-colors">
                  <td className="px-3 py-2.5 font-mono text-xs">{u.email}</td>
                  <td className="px-3 py-2.5">
                    {u.role === 'admin'
                      ? <Badge label="admin" variant="amber" />
                      : <Badge label="user" />}
                  </td>
                  <td className="px-3 py-2.5">
                    <Badge label={u.status} variant={u.status === 'active' ? 'green' : 'default'} />
                  </td>
                  <td className="px-3 py-2.5">
                    <Badge label={u.email_verified ? 'yes' : 'no'} variant={u.email_verified ? 'green' : 'amber'} />
                  </td>
                  <td className="px-3 py-2.5">
                    <Badge label={u.totp_enabled ? 'on' : 'off'} variant={u.totp_enabled ? 'green' : 'default'} />
                  </td>
                  <td className="px-3 py-2.5 text-muted-foreground">{formatDate(u.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* Orgs table */}
      <section>
        <h2 className="text-base font-semibold mb-3">Organizations</h2>
        <div className="rounded-lg border overflow-hidden">
          <table className="w-full text-sm">
            <TableHead cols={['Name', 'Slug', 'Plan', 'Members', 'Secrets', 'Created']} />
            <tbody>
              {orgs.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-3 py-6 text-center text-muted-foreground text-sm">
                    No organizations found.
                  </td>
                </tr>
              )}
              {orgs.map((o) => (
                <tr key={o.id} className="border-b last:border-0 hover:bg-muted/30 transition-colors">
                  <td className="px-3 py-2.5 font-medium">{o.name}</td>
                  <td className="px-3 py-2.5 font-mono text-xs text-muted-foreground">{o.slug}</td>
                  <td className="px-3 py-2.5">
                    <Badge label={o.plan} variant={planVariant(o.plan)} />
                  </td>
                  <td className="px-3 py-2.5">{o.member_count}</td>
                  <td className="px-3 py-2.5">{o.secrets_count}</td>
                  <td className="px-3 py-2.5 text-muted-foreground">{formatDate(o.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* Payments section */}
      <section>
        <div className="flex items-center gap-2 mb-3">
          <CreditCard className="w-4 h-4 text-muted-foreground" />
          <h2 className="text-base font-semibold">Stripe Subscriptions</h2>
        </div>
        {payments.length === 0 ? (
          <p className="text-sm text-muted-foreground">No orgs with Stripe subscriptions.</p>
        ) : (
          <div className="rounded-lg border overflow-hidden">
            <table className="w-full text-sm">
              <TableHead cols={['Org', 'Plan', 'Seats', 'Customer ID', 'Subscription ID']} />
              <tbody>
                {payments.map((p) => (
                  <tr key={p.org_id} className="border-b last:border-0 hover:bg-muted/30 transition-colors">
                    <td className="px-3 py-2.5 font-medium">{p.org_name}</td>
                    <td className="px-3 py-2.5">
                      <Badge label={p.plan} variant={planVariant(p.plan)} />
                    </td>
                    <td className="px-3 py-2.5">{p.plan_seats}</td>
                    <td className="px-3 py-2.5 font-mono text-xs text-muted-foreground">
                      {p.stripe_customer_id || '—'}
                    </td>
                    <td className="px-3 py-2.5 font-mono text-xs text-muted-foreground">
                      {p.stripe_subscription_id || '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  )
}
