'use client'

/**
 * BossCharacter — composite component that combines Three.js rendering,
 * animation state machine, HP bar, and boss name display.
 *
 * Exposes `triggerAttack`, `triggerDamage`, and `triggerDeath` via a forwarded
 * ref so parent components can drive animation from game logic.
 */

import { forwardRef, useImperativeHandle } from 'react'
import { BossCanvas } from './BossCanvas'
import { bossCatalog } from './bossCatalog'
import { useBossAnimation } from './useBossAnimation'
import type { AnimationState, BossId } from './types'

/** Methods exposed on the BossCharacter imperative handle. */
export interface BossCharacterHandle {
  /** Trigger the attack animation. */
  triggerAttack: () => void
  /** Trigger the damage animation. */
  triggerDamage: () => void
  /** Trigger the death animation (terminal). */
  triggerDeath: () => void
}

/** Props accepted by BossCharacter. */
export interface BossCharacterProps {
  /** Which boss to display. */
  bossId: BossId
  /** Current HP value (0 → maxHp). */
  hp: number
  /** Maximum HP value. */
  maxHp: number
  /** Called when a timed animation completes and transitions back to idle or ends at death. */
  onAnimationComplete?: (state: AnimationState) => void
}

/**
 * Fully self-contained boss character widget.
 * Renders the Three.js canvas, an HP bar, and the boss's display name.
 * Animation is controlled imperatively via the forwarded ref.
 */
export const BossCharacter = forwardRef<BossCharacterHandle, BossCharacterProps>(
  function BossCharacter({ bossId, hp, maxHp, onAnimationComplete }, ref) {
    const { state, progress, triggerAttack, triggerDamage, triggerDeath } =
      useBossAnimation()

    const config = bossCatalog.find((b) => b.id === bossId)!

    /** Clamp HP ratio to [0, 1]. */
    const hpRatio = maxHp > 0 ? Math.min(Math.max(hp / maxHp, 0), 1) : 0
    const hpPercent = Math.round(hpRatio * 100)

    // Expose imperative controls to parent via ref.
    useImperativeHandle(ref, () => ({
      triggerAttack,
      triggerDamage,
      triggerDeath,
    }))

    return (
      <div className="flex flex-col items-center gap-3">
        {/* Three.js scene */}
        <BossCanvas
          bossId={bossId}
          animState={state}
          animProgress={progress}
          width={320}
          height={320}
        />

        {/* Boss name */}
        <p
          className="text-lg font-bold tracking-widest uppercase"
          style={{ color: config.color }}
          data-testid="boss-name"
        >
          {config.name}
        </p>

        {/* HP bar */}
        <div
          className="w-full max-w-[320px] bg-gray-800 rounded-full h-4 overflow-hidden"
          role="progressbar"
          aria-label={`${config.name} HP`}
          aria-valuenow={hp}
          aria-valuemin={0}
          aria-valuemax={maxHp}
        >
          <div
            className="h-full rounded-full transition-all duration-300"
            style={{
              width: `${hpPercent}%`,
              backgroundColor: config.color,
              boxShadow: `0 0 8px ${config.color}`,
            }}
            data-testid="hp-bar-fill"
          />
        </div>

        {/* HP label */}
        <p className="text-xs text-gray-400">
          {hp} / {maxHp} HP
        </p>
      </div>
    )
  },
)
