'use client'

/**
 * Final step of the onboarding wizard — the completion screen.
 *
 * Signals to the user that setup is done and invites them into the main app.
 * The parent wizard marks onboarding complete before rendering this step.
 */

interface ReadyStepProps {
  /** The user's chosen avatar emoji, shown in the celebration graphic. */
  avatarEmoji: string
  /** The user's chosen display name, shown in the celebration heading. */
  displayName: string
  /** Called when the user clicks "Start learning". */
  onDone: () => void
}

/**
 * ReadyStep — completion screen shown at the end of the onboarding wizard.
 *
 * @param props.avatarEmoji - User's chosen avatar.
 * @param props.displayName - User's display name.
 * @param props.onDone - Navigate to the main tutoring interface.
 */
export default function ReadyStep({ avatarEmoji, displayName, onDone }: ReadyStepProps) {
  return (
    <div className="flex flex-col items-center text-center gap-6">
      <div className="relative">
        <div className="w-24 h-24 rounded-full bg-bg-card border-2 border-neon-gold shadow-neon-gold-sm flex items-center justify-center text-5xl animate-bounce-slow">
          {avatarEmoji}
        </div>
        {/* Sparkle ring */}
        <div className="absolute -inset-2 rounded-full border border-neon-gold/30 animate-ping" />
      </div>

      <div>
        <h2 className="font-mono text-2xl font-bold text-neon-gold mb-2">
          You&apos;re all set, {displayName}!
        </h2>
        <p className="text-sm text-text-base max-w-xs">
          Your AI tutor is waiting. Upload a study material or just start asking questions.
        </p>
      </div>

      <div className="flex flex-col gap-2 w-full max-w-xs text-xs text-text-dim">
        <p>✓ Character created</p>
        <p>✓ Materials upload ready</p>
        <p>✓ 14-day free trial active</p>
      </div>

      <button onClick={onDone} className="neon-btn-primary w-full max-w-xs text-base">
        Start learning ⚡
      </button>
    </div>
  )
}
