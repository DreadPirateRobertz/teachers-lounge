/**
 * BattleEffects — composed VFX wrapper for the Weird Science boss battle UI.
 *
 * Combines particle bursts, screen shake, molecule morph, death dissolution,
 * and combo streak badge into a single imperative handle.
 */

'use client'

import React, { forwardRef, useImperativeHandle, useRef, type ReactNode } from 'react'
import ParticleBurst, { type ParticleBurstHandle } from './ParticleBurst'
import { useScreenShake } from './useScreenShake'
import { useMoleculeMorph } from './useMoleculeMorph'
import { useDeathDissolution } from './useDeathDissolution'
import ComboStreakBadge from './ComboStreakBadge'

/** Pixel coordinates used to position a particle burst. */
export interface BurstOrigin {
  /** Horizontal pixel position. */
  x: number
  /** Vertical pixel position. */
  y: number
}

/** Imperative methods exposed by BattleEffects via forwardRef. */
export interface BattleEffectsHandle {
  /**
   * Triggers a correct-answer particle burst at the given origin.
   * @param origin - Screen-space pixel coordinates.
   */
  triggerCorrect(origin: BurstOrigin): void
  /**
   * Triggers a wrong-answer particle burst at the given origin.
   * @param origin - Screen-space pixel coordinates.
   */
  triggerWrong(origin: BurstOrigin): void
  /**
   * Triggers the screen-shake crit effect.
   * Uses default intensity 1.0.
   */
  triggerCrit(): void
  /**
   * Triggers the molecule morph geometry distortion animation.
   */
  triggerMorph(): void
  /**
   * Triggers the boss death dissolution animation.
   */
  triggerDissolve(): void
}

/** Props for the BattleEffects component. */
export interface BattleEffectsProps {
  /** Current consecutive-correct-answer count for combo streak display. */
  comboCount: number
  /** Battle UI content to wrap with effects. */
  children: ReactNode
  /** Forwarded ref for imperative control. */
  ref?: React.Ref<BattleEffectsHandle>
}

/**
 * Wraps battle UI children with all Weird Science VFX.
 *
 * The combo badge is positioned top-right of the wrapper. Screen shake is
 * applied as a CSS transform on the outer wrapper. Particle overlays are
 * rendered above all content.
 *
 * @example
 * ```tsx
 * const effectsRef = useRef<BattleEffectsHandle>(null)
 *
 * <BattleEffects comboCount={combo} ref={effectsRef}>
 *   <BossCanvas ... />
 * </BattleEffects>
 *
 * // On correct answer:
 * effectsRef.current?.triggerCorrect({ x: 200, y: 300 })
 * effectsRef.current?.triggerMorph()
 * ```
 */
const BattleEffects = forwardRef<BattleEffectsHandle, BattleEffectsProps>(function BattleEffects(
  { comboCount, children },
  ref,
) {
  const burstRef = useRef<ParticleBurstHandle>(null)
  const { shakeStyle, triggerShake } = useScreenShake()
  const { triggerMorph } = useMoleculeMorph()
  const { triggerDissolve } = useDeathDissolution()

  useImperativeHandle(ref, () => ({
    triggerCorrect(origin: BurstOrigin) {
      burstRef.current?.triggerCorrect(origin)
    },
    triggerWrong(origin: BurstOrigin) {
      burstRef.current?.triggerWrong(origin)
    },
    triggerCrit() {
      triggerShake(1.0)
    },
    triggerMorph() {
      triggerMorph()
    },
    triggerDissolve() {
      triggerDissolve()
    },
  }))

  return (
    <div
      style={{
        position: 'relative',
        width: '100%',
        height: '100%',
        transform: shakeStyle || undefined,
      }}
    >
      {/* Combo streak badge — top-right overlay */}
      <div
        style={{
          position: 'absolute',
          top: 12,
          right: 12,
          zIndex: 10,
        }}
      >
        <ComboStreakBadge comboCount={comboCount} />
      </div>

      {/* Particle burst overlay wrapping the main content */}
      <ParticleBurst ref={burstRef}>{children}</ParticleBurst>
    </div>
  )
})

BattleEffects.displayName = 'BattleEffects'

export default BattleEffects
