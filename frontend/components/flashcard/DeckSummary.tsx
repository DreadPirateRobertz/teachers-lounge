'use client'

// ── Types ────────────────────────────────────────────────────────────────────

interface Props {
  /** Total number of flashcards in the user's deck. */
  total: number
  /** Number of cards due for review today. */
  dueCount: number
  /** Called when the user clicks "Review Now". */
  onStartReview: () => void
  /** Called when the user clicks "Export to Anki". */
  onExportAnki: () => void
}

// ── Component ────────────────────────────────────────────────────────────────

/**
 * DeckSummary shows the user's deck stats (total cards, cards due today)
 * and provides actions to start a review session or export to Anki.
 */
export default function DeckSummary({ total, dueCount, onStartReview, onExportAnki }: Props) {
  const hasDue = dueCount > 0
  const hasCards = total > 0

  return (
    <div className="bg-bg-card border border-border-mid rounded-xl p-6 flex flex-col gap-5">
      {/* Stats row */}
      <div className="flex items-center justify-around">
        {/* Total */}
        <div className="flex flex-col items-center gap-1">
          <span className="font-mono text-2xl font-bold text-text-bright">{total}</span>
          <span className="text-[11px] text-text-dim uppercase tracking-wider">Total cards</span>
        </div>

        {/* Divider */}
        <div className="h-10 w-px bg-border-dim" />

        {/* Due today */}
        <div className="flex flex-col items-center gap-1">
          <span
            className={`font-mono text-2xl font-bold ${
              hasDue ? 'text-neon-blue' : 'text-text-dim'
            }`}
          >
            {dueCount}
          </span>
          <span className="text-[11px] text-text-dim uppercase tracking-wider">Due today</span>
          {hasDue && (
            <span className="text-[9px] text-neon-blue animate-pulse">● ready</span>
          )}
        </div>
      </div>

      {/* Action buttons */}
      <div className="flex flex-col gap-2">
        <button
          onClick={onStartReview}
          disabled={!hasDue}
          className="w-full py-2.5 bg-neon-blue text-bg-deep text-xs font-semibold rounded-xl hover:bg-neon-blue/90 transition-colors shadow-neon-blue disabled:opacity-40 disabled:cursor-not-allowed disabled:shadow-none"
        >
          {hasDue ? `Review Now (${dueCount})` : 'No cards due'}
        </button>

        <button
          onClick={onExportAnki}
          disabled={!hasCards}
          className="w-full py-2.5 border border-neon-gold/50 text-neon-gold text-xs font-semibold rounded-xl hover:bg-neon-gold/10 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
        >
          Export to Anki (.apkg)
        </button>
      </div>
    </div>
  )
}
