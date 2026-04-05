/**
 * Onboarding page — first-run setup wizard.
 *
 * Server component that gates access and prefetches the user's profile so
 * the wizard has current display name and avatar emoji on first render.
 *
 * Redirect behaviour:
 * - No auth token → middleware redirects to /login (no action needed here).
 * - has_completed_onboarding: true → redirect to / (already onboarded).
 * - has_completed_onboarding: false → render OnboardingWizard.
 */

import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import OnboardingWizard from '@/components/onboarding/OnboardingWizard'

const USER_SERVICE_URL = process.env.USER_SERVICE_URL || 'http://user-service:8080'

/**
 * Decode the `sub` claim from a JWT without signature verification.
 *
 * Safe here because we immediately use the ID to make an authenticated
 * request to the user-service, which performs full JWT validation.
 *
 * @param token - Raw JWT string.
 * @returns The `sub` claim string, or `null` on any parse failure.
 */
function extractSub(token: string): string | null {
  try {
    const payload = token.split('.')[1]
    return JSON.parse(Buffer.from(payload, 'base64url').toString()).sub ?? null
  } catch {
    return null
  }
}

/**
 * Fetch the authenticated user's profile from the user-service.
 *
 * @param userId - UUID of the user.
 * @param token - JWT access token for the Authorization header.
 * @returns The profile object, or `null` on any error.
 */
async function fetchProfile(userId: string, token: string) {
  try {
    const res = await fetch(`${USER_SERVICE_URL}/users/${userId}/profile`, {
      headers: { Authorization: `Bearer ${token}` },
      cache: 'no-store',
    })
    if (!res.ok) return null
    return res.json() as Promise<{
      user: {
        id: string
        display_name: string
        avatar_emoji: string
        has_completed_onboarding: boolean
      }
    }>
  } catch {
    return null
  }
}

/**
 * OnboardPage — server component entry point for the onboarding wizard.
 *
 * Checks whether the authenticated user has already completed onboarding and
 * redirects to `/` if so.  Otherwise renders the multi-step wizard.
 */
export default async function OnboardPage() {
  const cookieStore = await cookies()
  const token = cookieStore.get('tl_token')?.value

  if (!token) {
    redirect('/login')
  }

  const userId = extractSub(token)
  if (!userId) {
    redirect('/login')
  }

  const profile = await fetchProfile(userId, token)

  // If we can't fetch the profile (service down, token expired, etc.) let the
  // user see the wizard anyway — it's non-blocking.  If already onboarded,
  // skip to the main app.
  if (profile?.user.has_completed_onboarding) {
    redirect('/')
  }

  const displayName = profile?.user.display_name ?? 'Scholar'
  const avatarEmoji = profile?.user.avatar_emoji ?? '🎓'

  return (
    <main className="min-h-screen bg-bg-deep flex flex-col items-center justify-center px-4 py-12">
      <OnboardingWizard userId={userId} displayName={displayName} avatarEmoji={avatarEmoji} />
    </main>
  )
}
