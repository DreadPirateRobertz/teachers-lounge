'use client'

// ── Types ────────────────────────────────────────────────────────────────────

/** SM-2 quality rating: 0 = complete blackout, 5 = perfect recall */
type Quality = 0 | 1 | 2 | 3 | 4 | 5

interface Props {
  /** Called with the SM-2 quality value (0–5) when a rating button is clicked. */
  onRate: (quality: Quality) => void
  /** When true all buttons are rendered but non-interactive. */
  disabled?: boolean
}

// ── Button metadata ──────────────────────────────────────────────────────────

interface RatingMeta {
  quality: Quality
  label: string
  colorClass: string
  borderClass: string
  textClass: string
}

/**
 * SM-2 rating button descriptors.
 * 0-2 reset the repetition interval; 3-5 advance it.
 */
const RATINGS: RatingMeta[] = [
  {
    quality: 0,
    label: 'Blackout',
    colorClass: 'bg-neon-pink/20 hover:bg-neon-pink/30',
    borderClass: 'border-neon-pink/60',
    textClass: 'text-neon-pink',
  },
  {
    quality: 1,
    label: 'Wrong',
    colorClass: 'bg-neon-pink/10 hover:bg-neon-pink/20',
    borderClass: 'border-neon-pink/30',
    textClass: 'text-neon-pink/80',
  },
  {
    quality: 2,
    label: 'Hard',
    colorClass: 'bg-neon-gold/20 hover:bg-neon-gold/30',
    borderClass: 'border-neon-gold/60',
    textClass: 'text-neon-gold',
  },
  {
    quality: 3,
    label: 'OK',
    colorClass: 'bg-neon-gold/10 hover:bg-neon-gold/20',
    borderClass: 'border-neon-gold/30',
    textClass: 'text-neon-gold/80',
  },
  {
    quality: 4,
    label: 'Good',
    colorClass: 'bg-neon-green/10 hover:bg-neon-green/20',
    borderClass: 'border-neon-green/40',
    textClass: 'text-neon-green',
  },
  {
    quality: 5,
    label: 'Perfect',
    colorClass: 'bg-neon-green/20 hover:bg-neon-green/30',
    borderClass: 'border-neon-green/60',
    textClass: 'text-neon-green',
  },
]

// ── Component ────────────────────────────────────────────────────────────────

/**
 * RatingButtons renders six SM-2 quality rating buttons (0–5).
 *
 * The buttons are split into two groups with sub-labels explaining the
 * effect on the card's review interval:
 *  - 0-2: reset interval (red/gold)
 *  - 3-5: advance interval (green)
 */
export default function RatingButtons({ onRate, disabled = false }: Props) {
  return (
    <div className="w-full">
      {/* Buttons row */}
      <div className="grid grid-cols-6 gap-1">
        {RATINGS.map((r) => (
          <button
            key={r.quality}
            onClick={() => !disabled && onRate(r.quality)}
            disabled={disabled}
            aria-label={r.label}
            className={`
              flex flex-col items-center justify-center
              rounded-lg border px-1 py-2
              transition-all duration-150
              ${r.colorClass} ${r.borderClass}
              disabled:opacity-40 disabled:cursor-not-allowed
            `}
          >
            <span className={`font-mono text-xs font-bold ${r.textClass}`}>{r.quality}</span>
            <span className={`text-[10px] font-medium leading-tight mt-0.5 ${r.textClass}`}>
              {r.label}
            </span>
          </button>
        ))}
      </div>

      {/* Hint sub-labels */}
      <div className="flex justify-between mt-1 px-0.5">
        <span className="text-[10px] text-neon-pink/70">0-2 reset interval</span>
        <span className="text-[10px] text-neon-green/70">3-5 advance</span>
      </div>
    </div>
  )
}
