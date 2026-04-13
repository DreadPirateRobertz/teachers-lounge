'use client'

/**
 * BattleResolve.tsx
 *
 * Overlay shown during the "resolve" phase of a boss battle.
 * Displays the damage numbers dealt by both sides and the correct
 * answer explanation (if any). Auto-dismissed by the parent after a
 * fixed delay — this component is purely presentational.
 */

/** Props for BattleResolve. */
export interface BattleResolveProps {
  /** Damage dealt to the boss (0 on a wrong answer). */
  playerDamage: number
  /** Damage dealt to the player (0 if boss is dead). */
  bossDamage: number
  /** Whether the player answered correctly. */
  correct: boolean
  /** Optional explanation from the quiz API, shown after answering. */
  explanation?: string
}

/**
 * BattleResolve shows the round outcome: damage exchanged and whether the
 * player's answer was correct. Fully accessible via aria-live so screen
 * readers announce the result without needing visual focus.
 */
export default function BattleResolve({
  playerDamage,
  bossDamage,
  correct,
  explanation,
}: BattleResolveProps) {
  return (
    <div
      aria-live="polite"
      className="flex flex-col items-center gap-3 w-full max-w-md
                 rounded-xl border border-border-dim bg-bg-card/90 px-6 py-5"
    >
      {/* Outcome banner */}
      <div
        className="text-lg font-mono font-black tracking-widest"
        style={{
          color: correct ? '#00ff88' : '#ff3366',
          textShadow: correct ? '0 0 12px #00ff8866' : '0 0 12px #ff336666',
        }}
      >
        {correct ? '✓ CORRECT!' : '✗ WRONG'}
      </div>

      {/* Damage numbers */}
      <div className="flex gap-6 font-mono text-sm">
        {playerDamage > 0 && (
          <div className="flex flex-col items-center gap-0.5">
            <span className="text-xs text-text-dim uppercase tracking-wide">You dealt</span>
            <span className="text-neon-green font-bold text-xl">-{playerDamage}</span>
            <span className="text-[10px] text-text-dim">to boss</span>
          </div>
        )}
        {bossDamage > 0 && (
          <div className="flex flex-col items-center gap-0.5">
            <span className="text-xs text-text-dim uppercase tracking-wide">Boss dealt</span>
            <span className="text-neon-pink font-bold text-xl">-{bossDamage}</span>
            <span className="text-[10px] text-text-dim">to you</span>
          </div>
        )}
        {playerDamage === 0 && bossDamage === 0 && (
          <span className="text-text-dim text-xs">No damage this round</span>
        )}
      </div>

      {/* Explanation */}
      {explanation && (
        <p className="text-xs font-mono text-text-dim leading-relaxed text-center border-t border-border-dim pt-3 mt-1">
          {explanation}
        </p>
      )}
    </div>
  )
}
