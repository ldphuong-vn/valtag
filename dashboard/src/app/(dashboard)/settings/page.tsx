import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import { getSession } from '@/lib/auth'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import ProfileCard from './profile-card'
import ChannelsCard from './channels-card'
import ChangePasswordCard from './change-password-card'
import SignOutButton from './sign-out-button'

// Settings: profile + contact channels + security. (Usage was removed
// 2026-11-21 — valtag has no plans/quotations to sell; usage counters live
// with the ops metrics, not in the user-facing settings.)
export default async function SettingsPage() {
  const cookieStore = await cookies()
  const token = cookieStore.get('valt_access_token')?.value
  const session = token ? await getSession() : null
  if (!session) redirect('/login')

  return (
    <div className="max-w-2xl space-y-6">
      <h2 className="text-lg font-semibold">Settings</h2>

      <ProfileCard fallbackId={session.id} />

      <ChannelsCard />

      <ChangePasswordCard />

      <Card>
        <CardHeader><CardTitle className="text-base">Account</CardTitle></CardHeader>
        <CardContent>
          <SignOutButton />
        </CardContent>
      </Card>
    </div>
  )
}
