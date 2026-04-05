'use client'

/**
 * First step of the onboarding wizard.
 *
 * Introduces the product and sets expectations for the remaining wizard
 * steps so new users arrive at the tutoring interface with context.
 */

interface WelcomeStepProps {
  /** Display name captured at registration, shown in the greeting. */
  displayName: string
  /** Called when the user clicks "Let's go". */
  onNext: () => void
}

const FEATURE_BULLETS = [
  { emoji: '🧠', label: 'AI tutor that adapts to your learning style' },
  { emoji: '⚔️', label: 'Boss battles to test your knowledge' },
  { emoji: '📚', label: 'Upload any study material — PDF, notes, slides' },
  { emoji: '🏆', label: 'Quests, XP, and a leaderboard to keep you sharp' },
]

/**
 * WelcomeStep — introduction card for the onboarding wizard.
 *
 * @param props.displayName - User's chosen display name.
 * @param props.onNext - Advance to the next wizard step.
 */
export default function WelcomeStep({ displayName, onNext }: WelcomeStepProps) {
  return (
    <div className="flex flex-col items-center text-center gap-6">
      <div className="text-6xl animate-bounce-slow">🎓</div>

      <div>
        <h1 className="font-mono text-2xl font-bold text-neon-green text-glow-green mb-2">
          Welcome, {displayName}!
        </h1>
        <p className="text-sm text-text-base">
          Your AI-powered tutor is ready. Here&apos;s what you can do:
        </p>
      </div>

      <ul className="w-full max-w-xs text-left flex flex-col gap-3">
        {FEATURE_BULLETS.map(({ emoji, label }) => (
          <li key={label} className="flex items-start gap-3 text-sm text-text-base">
            <span className="text-lg leading-none mt-0.5">{emoji}</span>
            <span>{label}</span>
          </li>
        ))}
      </ul>

      <button onClick={onNext} className="neon-btn-primary w-full max-w-xs">
        Let&apos;s go ⚡
      </button>
    </div>
  )
}
