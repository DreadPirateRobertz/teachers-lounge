'use client'

import RatingButtons from '@/components/flashcard/RatingButtons'

// ── Types ────────────────────────────────────────────────────────────────────

/** SM-2 flashcard as returned by the backend. */
export interface Flashcard {
  id: string
  user_id: string
  question_id?: string
  session_id?: string
  front: string
  back: string
  source: string
  topic?: string
  course_id?: string
  ease_factor: number
  interval_days: number
  repetitions: number
  next_review_at: string
  last_reviewed_at?: string
  created_at: string
}

/** SM-2 quality rating value. */
type Quality = 0 | 1 | 2 | 3 | 4 | 5

interface Props {
  /** The flashcard to display. */
  card: Flashcard
  /** Called with quality 0–5 when the user rates the card. */
  onRate: (quality: Quality) => void
  /** Whether the back of the card is currently visible. */
  isRevealed: boolean
  /** Called when the user requests to see the back of the card. */
  onReveal: () => void
}

// ── Helpers ──────────────────────────────────────────────────────────────────

/**
 * Returns Tailwind border and shadow classes based on the card's ease_factor.
 *
 * - ef >= 2.5 → neon-blue (good)
 * - ef >= 1.8 → neon-gold (fair)
 * - ef <  1.8 → neon-pink (struggling)
 */
function easeBorderClass(easeFactor: number): string {
  if (easeFactor >= 2.5) return 'border-neon-blue shadow-neon-blue'
  if (easeFactor >= 1.8) return 'border-neon-gold'
  return 'border-neon-pink'
}

/**
 * Returns an inline style object for the card face wrapper that applies
 * the 3-D CSS flip transform.
 */
function faceStyle(revealed: boolean, back: boolean): React.CSSProperties {
  const base: React.CSSProperties = {
    position: 'absolute',
    inset: 0,
    backfaceVisibility: 'hidden',
    WebkitBackfaceVisibility: 'hidden',
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    padding: '2rem',
    borderRadius: '0.75rem',
    transition: 'transform 0.55s cubic-bezier(0.4, 0.2, 0.2, 1)',
  }
  if (!back) {
    return {
      ...base,
      transform: revealed ? 'rotateY(180deg)' : 'rotateY(0deg)',
    }
  }
  return {
    ...base,
    transform: revealed ? 'rotateY(0deg)' : 'rotateY(-180deg)',
  }
}

// ── Component ────────────────────────────────────────────────────────────────

/**
 * FlashCard renders a single SM-2 flashcard with a 3-D CSS flip animation.
 *
 * The front shows the question; the back shows the answer plus SM-2 rating
 * buttons. Clicking the card (or the reveal button) triggers onReveal.
 * The card border colour reflects the ease_factor: blue (good), gold (fair),
 * pink (struggling).
 */
export default function FlashCard({ card, onRate, isRevealed, onReveal }: Props) {
  const borderClass = easeBorderClass(card.ease_factor)

  return (
    <div className="w-full flex flex-col gap-4">
      {/* Card container */}
      <div
        role="button"
        tabIndex={0}
        aria-label={isRevealed ? 'Card revealed' : 'Click to reveal answer'}
        onClick={() => !isRevealed && onReveal()}
        onKeyDown={(e) => {
          if (!isRevealed && (e.key === 'Enter' || e.key === ' ')) {
            e.preventDefault()
            onReveal()
          }
        }}
        className={`
          relative w-full rounded-xl border-2 bg-bg-card
          ${borderClass}
          ${!isRevealed ? 'cursor-pointer hover:scale-[1.01] transition-transform' : 'cursor-default'}
        `}
        style={{
          perspective: '1000px',
          minHeight: '220px',
        }}
      >
        {/* Inner 3-D container */}
        <div
          style={{
            position: 'relative',
            width: '100%',
            height: '220px',
            transformStyle: 'preserve-3d',
          }}
        >
          {/* Front face */}
          <div style={faceStyle(isRevealed, false)}>
            <p className="text-sm font-medium text-text-bright text-center leading-relaxed">
              {card.front}
            </p>
            {!isRevealed && (
              <span className="mt-4 text-[10px] text-text-dim">tap to reveal</span>
            )}
          </div>

          {/* Back face */}
          <div style={faceStyle(isRevealed, true)}>
            <p className="text-sm text-text-base text-center leading-relaxed">{card.back}</p>
          </div>
        </div>
      </div>

      {/* Reveal button — only shown when not yet revealed */}
      {!isRevealed && (
        <button
          onClick={onReveal}
          className="w-full py-2.5 border border-border-mid rounded-xl text-xs font-semibold text-text-base hover:border-neon-blue/40 hover:text-neon-blue transition-colors bg-bg-panel"
        >
          Show Answer
        </button>
      )}

      {/* Rating buttons — only shown after reveal */}
      {isRevealed && (
        <div className="animate-fade-in">
          <p className="text-[10px] text-text-dim text-center mb-2">How well did you recall this?</p>
          <RatingButtons onRate={onRate} />
        </div>
      )}
    </div>
  )
}
