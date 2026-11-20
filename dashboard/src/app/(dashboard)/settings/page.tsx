import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import { getSession } from '@/lib/auth'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import ProfileCard from './profile-card'
import NotificationsChannelsCard from './notifications-channels-card'
import ChangePasswordCard from './change-password-card'
import SignOutButton from './sign-out-button'

// Settings: profile + notification channels (config, not just a mirror) +
// security. The separate /settings/notifications page was folded in here
// 2026-11-21. (Usage was removed — valtag has no plans to sell; usage
// counters live with the ops metrics, not in user-facing settings.)
export default async function SettingsPage() {
  const cookieStore = await cookies()
  const token = cookieStore.get('valt_access_token')?.value
  const session = token ? await getSession() : null
  if (!session) redirect('/login')

  return (
    <div className="max-w-2xl space-y-6">
      <h2 className="text-lg font-semibold">Settings</h2>

      <ProfileCard fallbackId={session.id} />

      <NotificationsChannelsCard />

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
