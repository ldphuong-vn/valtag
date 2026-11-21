'use client'

import { useEffect, useState, Suspense } from 'react'
import Link from 'next/link'
import { useSearchParams } from 'next/navigation'
import { Building2, Mail, ShieldCheck } from 'lucide-react'

// Accept-invite landing: an operator/admin shares /accept-invite?token=… —
// this page shows who invited and lets the invitee create their account
// (register carries the invite token server-side and joins the org).
function AcceptInviteInner() {
  const searchParams = useSearchParams()
  const token = searchParams.get('token') ?? ''
  const [invite, setInvite] = useState<{ org_name: string; email: string; role: string } | null>(null)
  const [error, setError] = useState('')
  const [loggedIn, setLoggedIn] = useState<boolean | null>(null)

  useEffect(() => {
    if (!token) {
      setError('Missing invitation token.')
      return
    }
    fetch(`/api/proxy/orgs/invitations/${token}`)
      .then(async r => {
        if (!r.ok) {
          const body = await r.json().catch(() => null)
          throw new Error(body?.error?.message ?? 'Invalid invitation')
        }
        return r.json()
      })
      .then(setInvite)
      .catch(e => setError(e instanceof Error ? e.message : 'Invalid invitation'))
    fetch('/api/v1/auth/me', { credentials: 'include' })
      .then(r => setLoggedIn(r.ok))
      .catch(() => setLoggedIn(false))
  }, [token])

  const registerHref = invite
    ? `/login?mode=register&invite=${token}&email=${encodeURIComponent(invite.email)}`
    : '/login?mode=register'

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen gap-3 text-muted-foreground px-4 text-center">
        <p className="text-lg font-semibold text-destructive">{error}</p>
        <Link href="/" className="text-sm text-primary hover:underline">Back to home</Link>
      </div>
    )
  }

  return (
    <div className="flex flex-col items-center justify-center min-h-screen px-4">
      <div className="w-full max-w-md border rounded-xl bg-card p-8 text-center space-y-4">
        <div className="w-12 h-12 mx-auto rounded-xl bg-primary/10 flex items-center justify-center">
          <Building2 className="w-6 h-6 text-primary" />
        </div>
        {invite ? (
          <>
            <h1 className="text-lg font-semibold">You&apos;re invited to join</h1>
            <p className="text-xl font-bold">{invite.org_name}</p>
            <div className="text-sm text-muted-foreground space-y-1">
              <p className="flex items-center justify-center gap-1.5"><Mail className="w-3.5 h-3.5" /> {invite.email}</p>
              <p className="flex items-center justify-center gap-1.5"><ShieldCheck className="w-3.5 h-3.5" /> Role: <span className="capitalize">{invite.role}</span></p>
            </div>
            {loggedIn ? (
              <p className="text-sm text-muted-foreground">You&apos;re signed in — the invitation is accepted from the Overview page.</p>
            ) : (
              <Link
                href={registerHref}
                className="inline-flex items-center justify-center w-full h-11 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
              >
                Create account &amp; join
              </Link>
            )}
          </>
        ) : (
          <p className="text-sm text-muted-foreground">Loading invitation…</p>
        )}
      </div>
    </div>
  )
}

export default function AcceptInvitePage() {
  return (
    <Suspense fallback={null}>
      <AcceptInviteInner />
    </Suspense>
  )
}
