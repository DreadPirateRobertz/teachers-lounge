/**
 * useComboStreak — reactive combo multiplier state derived from comboCount.
 *
 * Computes streak level, label, glow color, and active state based on how many
 * consecutive correct answers the player has given.
 */

'use client'

import { useMemo } from 'react'

/** Possible streak multiplier levels. */
export type StreakLevel = 0 | 1 | 2

/** Return type of the useComboStreak hook. */
export interface UseComboStreakReturn {
  /**
   * Current streak level:
   * - 0 — no streak (comboCount < 3)
   * - 1 — 1.5× multiplier (comboCount >= 3)
   * - 2 — 2× multiplier (comboCount >= 5)
   */
  streakLevel: StreakLevel
  /**
   * Human-readable combo label.
   * - '' at level 0
   * - '1.5× COMBO' at level 1
   * - '2× COMBO' at level 2
   */
  streakLabel: string
  /**
   * CSS color string for the neon glow border.
   * - '' at level 0
   * - '#00aaff' (neon-blue) at level 1
   * - '#ffdc00' (neon-gold) at level 2
   */
  glowColor: string
  /** True when comboCount >= 3. */
  isActive: boolean
}

/** Thresholds for each streak level. */
const LEVEL_1_THRESHOLD = 3
const LEVEL_2_THRESHOLD = 5

/** Neon color tokens for combo glow. */
const NEON_BLUE = '#00aaff'
const NEON_GOLD = '#ffdc00'

/**
 * Derives combo streak metadata from a reactive comboCount value.
 *
 * @param comboCount - Number of consecutive correct answers.
 * @returns Streak level, label, glow color, and active flag.
 *
 * @example
 * ```tsx
 * const { streakLevel, streakLabel, glowColor, isActive } = useComboStreak(comboCount)
 * ```
 */
export function useComboStreak(comboCount: number): UseComboStreakReturn {
  return useMemo<UseComboStreakReturn>(() => {
    let streakLevel: StreakLevel = 0
    if (comboCount >= LEVEL_2_THRESHOLD) {
      streakLevel = 2
    } else if (comboCount >= LEVEL_1_THRESHOLD) {
      streakLevel = 1
    }

    const labelMap: Record<StreakLevel, string> = {
      0: '',
      1: '1.5× COMBO',
      2: '2× COMBO',
    }

    const colorMap: Record<StreakLevel, string> = {
      0: '',
      1: NEON_BLUE,
      2: NEON_GOLD,
    }

    return {
      streakLevel,
      streakLabel: labelMap[streakLevel],
      glowColor: colorMap[streakLevel],
      isActive: comboCount >= LEVEL_1_THRESHOLD,
    }
  }, [comboCount])
}
