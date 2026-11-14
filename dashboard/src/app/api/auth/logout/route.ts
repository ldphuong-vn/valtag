import { cookies } from 'next/headers'
import { NextResponse } from 'next/server'

// The client handles navigation after logout. Redirecting to an absolute URL
// built from build-time env broke the button on domain deployments: the
// redirect targeted http://localhost:3000/login, and a cross-origin redirect
// under credentials 'same-origin' makes the calling fetch() fail silently
// (dogfood 2026-11-13 — sign out did nothing on valt.turbo.ai.vn).
export async function POST() {
  const cookieStore = await cookies()
  cookieStore.delete('valt_access_token')
  cookieStore.delete('valt_refresh_token')
  return NextResponse.json({ ok: true })
}
