'use client'

import { useState, useEffect, Suspense } from 'react'
import { Shield, Lock, Mail } from 'lucide-react'
import Link from 'next/link'
import { useSearchParams } from 'next/navigation'

// Helper: call /api/auth/set-tokens to store tokens in httpOnly cookies (XSS protection).
async function storeTokens(data: { access_token: string; refresh_token: string; expires_in?: number }) {
  await fetch('/api/auth/set-tokens', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
}

function LoginForm() {
  const searchParams = useSearchParams()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [info, setInfo] = useState('')
  const [loading, setLoading] = useState(false)
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [totpRequired, setTotpRequired] = useState(false)
  const [totpToken, setTotpToken] = useState('')
  const [totpCode, setTotpCode] = useState('')

  useEffect(() => {
    if (searchParams.get('reset') === 'success') {
      setInfo('Password reset successful. You can now sign in with your new password.')
    }
  }, [searchParams])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setInfo('')
    setLoading(true)

    try {
      const endpoint = mode === 'register' ? '/api/v1/auth/register' : '/api/v1/auth/login'
      const body: Record<string, string> = { email, password }
      if (mode === 'register') body.region_code = 'vn'

      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      const data = await res.json()

      if (!res.ok) {
        setError(data.error?.message ?? 'Authentication failed')
        return
      }

      // Check if TOTP is required
      if (data.requires_totp) {
        setTotpRequired(true)
        setTotpToken(data.totp_token)
        return
      }

      // Store tokens via server route to set httpOnly cookies (XSS protection)
      if (data.access_token) {
        await storeTokens(data)
        // New users (no projects yet) go to onboarding wizard; others go to dashboard root.
        window.location.href = data.needs_onboarding ? '/onboarding' : '/'
      }
    } catch {
      setError('Network error')
    } finally {
      setLoading(false)
    }
  }

  async function handleTotpSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const res = await fetch('/api/v1/auth/totp/validate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ totp_token: totpToken, code: totpCode }),
      })
      const data = await res.json()

      if (!res.ok) {
        setError(data.error?.message ?? 'Invalid code')
        return
      }

      if (data.access_token) {
        await storeTokens(data)
        window.location.href = '/'
      }
    } catch {
      setError('Network error')
    } finally {
      setLoading(false)
    }
  }

  // TOTP verification step
  if (totpRequired) {
    return (
      <div className="flex flex-col items-center gap-8">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center">
            <Shield className="w-5 h-5 text-primary" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">valtag</h1>
            <p className="text-xs text-muted-foreground">AI Secret Vault</p>
          </div>
        </div>

        <div className="w-full rounded-2xl border bg-card p-8 shadow-lg">
          <div className="text-center mb-6">
            <Lock className="w-8 h-8 mx-auto mb-4 text-primary" />
            <h2 className="text-2xl font-bold">Two-Factor Authentication</h2>
            <p className="text-muted-foreground mt-2 text-sm">Enter your authenticator code or backup code</p>
          </div>

          <form onSubmit={handleTotpSubmit} className="space-y-4">
            <input
              type="text"
              value={totpCode}
              onChange={e => setTotpCode(e.target.value)}
              placeholder="6-digit code"
              className="w-full px-4 py-3 rounded-lg border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-primary text-center tracking-widest text-lg"
              autoFocus
              maxLength={8}
            />
            {error && <p className="text-sm text-destructive text-center">{error}</p>}
            <button
              type="submit"
              disabled={loading || !totpCode}
              className="w-full rounded-lg bg-primary text-primary-foreground px-4 py-3 text-sm font-medium hover:bg-primary/90 disabled:opacity-50"
            >
              {loading ? 'Verifying...' : 'Verify'}
            </button>
          </form>
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col items-center gap-8">
      <div className="flex items-center gap-3">
        <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center">
          <Shield className="w-5 h-5 text-primary" />
        </div>
        <div>
          <h1 className="text-xl font-semibold tracking-tight">valtag</h1>
          <p className="text-xs text-muted-foreground">AI Secret Vault</p>
        </div>
      </div>

      <div className="w-full rounded-2xl border bg-card p-8 shadow-lg">
        <div className="text-center mb-6">
          <Lock className="w-8 h-8 mx-auto mb-4 text-primary" />
          <h2 className="text-2xl font-bold">{mode === 'login' ? 'Welcome back' : 'Create account'}</h2>
          <p className="text-muted-foreground mt-2 text-sm">Secure secret management for AI agents</p>
        </div>

        {/* Google OAuth */}
        <a
          href="/api/v1/auth/google"
          className="flex items-center justify-center gap-3 w-full rounded-lg border bg-background hover:bg-accent transition-colors px-4 py-3 text-sm font-medium"
        >
          <svg className="w-5 h-5" viewBox="0 0 24 24">
            <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
            <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
            <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
            <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
          </svg>
          Sign in with Google
        </a>

        <div className="relative my-6">
          <div className="absolute inset-0 flex items-center"><div className="w-full border-t" /></div>
          <div className="relative flex justify-center text-xs uppercase">
            <span className="bg-card px-2 text-muted-foreground">or</span>
          </div>
        </div>

        {info && (
          <div className="mb-4 p-3 rounded-lg bg-green-50 border border-green-200 text-green-800 text-sm text-center">
            {info}
          </div>
        )}

        {/* Email/Password form */}
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <div className="relative">
              <Mail className="absolute left-3 top-3 h-4 w-4 text-muted-foreground" />
              <input
                type="email"
                value={email}
                onChange={e => setEmail(e.target.value)}
                placeholder="Email"
                required
                className="w-full pl-10 pr-4 py-3 rounded-lg border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
          </div>
          <div>
            <div className="relative">
              <Lock className="absolute left-3 top-3 h-4 w-4 text-muted-foreground" />
              <input
                type="password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                placeholder="Password"
                required
                minLength={8}
                className="w-full pl-10 pr-4 py-3 rounded-lg border bg-background text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
            {mode === 'login' && (
              <div className="flex justify-end mt-1">
                <Link
                  href="/forgot-password"
                  className="text-xs text-muted-foreground hover:text-primary hover:underline"
                >
                  Forgot password?
                </Link>
              </div>
            )}
          </div>

          {error && <p className="text-sm text-destructive text-center">{error}</p>}

          <button
            type="submit"
            disabled={loading}
            className="w-full rounded-lg bg-primary text-primary-foreground px-4 py-3 text-sm font-medium hover:bg-primary/90 disabled:opacity-50"
          >
            {loading ? 'Loading...' : mode === 'login' ? 'Sign in' : 'Create account'}
          </button>
        </form>

        <p className="text-center text-sm text-muted-foreground mt-4">
          {mode === 'login' ? (
            <>No account?{' '}<button onClick={() => { setMode('register'); setError(''); setInfo('') }} className="text-primary hover:underline">Create one</button></>
          ) : (
            <>Have an account?{' '}<button onClick={() => { setMode('login'); setError(''); setInfo('') }} className="text-primary hover:underline">Sign in</button></>
          )}
        </p>
      </div>

      <p className="text-xs text-muted-foreground text-center">
        Human-in-the-loop approval for all AI agent secret access
      </p>
    </div>
  )
}

export default function LoginPage() {
  return (
    <Suspense fallback={
      <div className="flex items-center justify-center min-h-[200px]">
        <div className="text-muted-foreground text-sm">Loading...</div>
      </div>
    }>
      <LoginForm />
    </Suspense>
  )
}
