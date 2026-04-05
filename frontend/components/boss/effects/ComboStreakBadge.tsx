/**
 * ComboStreakBadge — pulsing neon badge that displays the current combo multiplier.
 *
 * Hidden when streakLevel is 0. Shows a neon-bordered badge with the combo label
 * and a Tailwind animate-pulse on active combos.
 */

'use client'

import React from 'react'
import { useComboStreak } from './useComboStreak'

/** Props for the ComboStreakBadge component. */
export interface ComboStreakBadgeProps {
  /** Number of consecutive correct answers; drives streak level calculation. */
  comboCount: number
}

/**
 * Displays a neon glow badge with the combo multiplier label.
 * The badge is invisible (not rendered) when streakLevel is 0.
 *
 * @example
 * ```tsx
 * <ComboStreakBadge comboCount={comboCount} />
 * ```
 */
export function ComboStreakBadge({ comboCount }: ComboStreakBadgeProps) {
  const { streakLevel, streakLabel, glowColor, isActive } =
    useComboStreak(comboCount)

  if (streakLevel === 0) {
    return null
  }

  return (
    <div
      aria-label={streakLabel}
      className={`inline-flex items-center justify-center px-3 py-1 rounded-full font-bold text-sm tracking-widest text-white select-none${isActive ? ' animate-pulse' : ''}`}
      style={{
        border: `2px solid ${glowColor}`,
        boxShadow: `0 0 8px ${glowColor}, 0 0 16px ${glowColor}`,
        background: 'rgba(10, 10, 26, 0.85)',
        color: glowColor,
      }}
    >
      {streakLabel}
    </div>
  )
}

export default ComboStreakBadge
