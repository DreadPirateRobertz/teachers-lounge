'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import WelcomeStep from './WelcomeStep'
import CharacterStep from './CharacterStep'
import UploadGuideStep from './UploadGuideStep'
import ReadyStep from './ReadyStep'

/**
 * Wizard step identifiers in display order.
 *
 * `welcome` → `character` → `upload-guide` → `ready`
 */
type Step = 'welcome' | 'character' | 'upload-guide' | 'ready'

const STEPS: Step[] = ['welcome', 'character', 'upload-guide', 'ready']

interface OnboardingWizardProps {
  /** User's ID — used to call PATCH /api/user/profile/:id/preferences. */
  userId: string
  /** Display name captured at registration. */
  displayName: string
  /** Current avatar emoji (from user record). */
  avatarEmoji: string
}

/**
 * OnboardingWizard — multi-step first-run setup wizard.
 *
 * Orchestrates four steps:
 * 1. Welcome — product intro
 * 2. Character creation — avatar + display name
 * 3. Upload guide — materials walkthrough
 * 4. Ready — completion screen
 *
 * On the character step the wizard persists avatar/name via the preferences
 * API.  After the ready step it calls `/api/user/onboarding` to mark the
 * wizard as complete, then navigates to `/`.
 *
 * @param props.userId - Authenticated user's UUID.
 * @param props.displayName - Initial display name from registration.
 * @param props.avatarEmoji - Initial avatar emoji.
 */
export default function OnboardingWizard({
  userId,
  displayName,
  avatarEmoji,
}: OnboardingWizardProps) {
  const router = useRouter()
  const [stepIndex, setStepIndex] = useState(0)
  const [currentName, setCurrentName] = useState(displayName)
  const [currentAvatar, setCurrentAvatar] = useState(avatarEmoji || '🎓')
  const [completionError, setCompletionError] = useState<string | null>(null)

  const step = STEPS[stepIndex]
  const totalSteps = STEPS.length

  /** Advance to the next wizard step. */
  function goNext() {
    setStepIndex((i) => Math.min(i + 1, STEPS.length - 1))
  }

  /**
   * Save character preferences then advance to the next step.
   *
   * @param name - Updated display name chosen by the user.
   * @param emoji - Selected avatar emoji.
   */
  async function saveCharacter(name: string, emoji: string) {
    const res = await fetch(`/api/user/profile/${userId}/preferences`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ display_name: name, avatar_emoji: emoji }),
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new Error((body as { error?: string }).error || 'Failed to save preferences')
    }
    setCurrentName(name)
    setCurrentAvatar(emoji)
    goNext()
  }

  /**
   * Mark onboarding as complete in the user-service then navigate to `/`.
   *
   * Called when the user clicks "Start learning" on the ReadyStep.
   */
  async function finishOnboarding() {
    setCompletionError(null)
    try {
      const res = await fetch('/api/user/onboarding', { method: 'PATCH' })
      if (!res.ok && res.status !== 204) {
        setCompletionError('Could not save progress — you can continue anyway.')
      }
    } catch {
      setCompletionError('Network error — you can continue anyway.')
    }
    router.push('/')
    router.refresh()
  }

  return (
    <div className="w-full max-w-sm mx-auto">
      {/* Step progress dots */}
      <div
        className="flex justify-center gap-2 mb-8"
        role="progressbar"
        aria-valuenow={stepIndex + 1}
        aria-valuemax={totalSteps}
      >
        {STEPS.map((s, i) => (
          <div
            key={s}
            aria-label={`Step ${i + 1} of ${totalSteps}${i === stepIndex ? ' (current)' : i < stepIndex ? ' (complete)' : ''}`}
            className={`w-2 h-2 rounded-full transition-colors ${
              i < stepIndex
                ? 'bg-neon-green'
                : i === stepIndex
                  ? 'bg-neon-blue shadow-neon-blue-sm'
                  : 'bg-border-mid'
            }`}
          />
        ))}
      </div>

      {/* Step content */}
      <div className="bg-bg-card border border-border-mid rounded-xl p-6 shadow-neon-blue">
        {step === 'welcome' && <WelcomeStep displayName={currentName} onNext={goNext} />}
        {step === 'character' && (
          <CharacterStep
            displayName={currentName}
            avatarEmoji={currentAvatar}
            onNext={saveCharacter}
          />
        )}
        {step === 'upload-guide' && <UploadGuideStep onNext={goNext} />}
        {step === 'ready' && (
          <ReadyStep
            avatarEmoji={currentAvatar}
            displayName={currentName}
            onDone={finishOnboarding}
          />
        )}
      </div>

      {completionError && (
        <p className="mt-3 text-xs text-red-400 text-center">{completionError}</p>
      )}
    </div>
  )
}
