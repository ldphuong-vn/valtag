'use client'

import { useState } from 'react'
import { Shield, Mail } from 'lucide-react'
import Link from 'next/link'

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState('')
  const [loading, setLoading] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const res = await fetch('/api/v1/auth/forgot-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email }),
      })

      if (!res.ok) {
        // Should not happen (server always returns 200), but handle gracefully.
        const data = await res.json().catch(() => ({}))
        setError(data?.error?.message ?? 'Something went wrong. Please try again.')
        return
      }

      setSubmitted(true)
    } catch {
      setError('Network error. Please try again.')
    } finally {
      setLoading(false)
    }
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
        {submitted ? (
          <div className="text-center space-y-4">
            <div className="w-12 h-12 rounded-full bg-green-100 flex items-center justify-center mx-auto">
              <Mail className="w-6 h-6 text-green-600" />
            </div>
            <h2 className="text-2xl font-bold">Check your email</h2>
            <p className="text-muted-foreground text-sm">
              If an account exists for <strong>{email}</strong>, we sent a password reset link.
              The link expires in 1 hour.
            </p>
            <p className="text-xs text-muted-foreground">
              Didn&apos;t receive it? Check your spam folder or{' '}
              <button
                onClick={() => setSubmitted(false)}
                className="text-primary hover:underline"
              >
                try again
              </button>
              .
            </p>
          </div>
        ) : (
          <>
            <div className="text-center mb-6">
              <h2 className="text-2xl font-bold">Forgot password?</h2>
              <p className="text-muted-foreground mt-2 text-sm">
                Enter your email and we&apos;ll send you a reset link.
              </p>
            </div>

            <form onSubmit={handleSubmit} className="space-y-4">
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

              {error && <p className="text-sm text-destructive text-center">{error}</p>}

              <button
                type="submit"
                disabled={loading}
                className="w-full rounded-lg bg-primary text-primary-foreground px-4 py-3 text-sm font-medium hover:bg-primary/90 disabled:opacity-50"
              >
                {loading ? 'Sending...' : 'Send reset link'}
              </button>
            </form>
          </>
        )}

        <p className="text-center text-sm text-muted-foreground mt-6">
          <Link href="/login" className="text-primary hover:underline">
            Back to sign in
          </Link>
        </p>
      </div>
    </div>
  )
}
