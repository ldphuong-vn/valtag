import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import { getSession } from '@/lib/auth'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import ProfileCard from './profile-card'
import ChangePasswordCard from './change-password-card'
import UsageCard from './usage-card'
import SignOutButton from './sign-out-button'

export default async function SettingsPage() {
  const cookieStore = await cookies()
  const token = cookieStore.get('valt_access_token')?.value
  const session = token ? await getSession() : null
  if (!session) redirect('/login')

  return (
    <div className="max-w-lg space-y-4">
      <h2 className="text-lg font-semibold">Settings</h2>

      <ProfileCard fallbackId={session.id} />

      <UsageCard />

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
