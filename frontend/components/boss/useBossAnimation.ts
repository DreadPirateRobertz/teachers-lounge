/**
 * useBossAnimation — React hook that drives the boss animation state machine.
 *
 * State transitions:
 *   idle  → attack → idle   (attack lasts 600 ms)
 *   idle  → damage → idle   (damage lasts 400 ms)
 *   any   → death           (terminal — no further transitions)
 *
 * `progress` is a 0→1 float that ticks up over the duration of the active
 * state, driven by requestAnimationFrame. In idle it stays at 0.
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import type { AnimationState } from './types'

/** Duration constants (ms) for each timed state. */
const DURATIONS: Record<Exclude<AnimationState, 'idle'>, number> = {
  attack: 600,
  damage: 400,
  death: 1200,
}

/** Return value from useBossAnimation. */
export interface BossAnimationControls {
  /** Current animation state. */
  state: AnimationState
  /** Progress through the current timed state: 0–1. Always 0 in idle. */
  progress: number
  /** Transition to 'attack' (no-op if dead). */
  triggerAttack: () => void
  /** Transition to 'damage' (no-op if dead). */
  triggerDamage: () => void
  /** Transition to 'death' (terminal). */
  triggerDeath: () => void
}

/**
 * Manages the boss animation state machine and returns controls + reactive
 * state for use by rendering components.
 */
export function useBossAnimation(): BossAnimationControls {
  const [state, setState] = useState<AnimationState>('idle')
  const [progress, setProgress] = useState(0)

  /** Tracks whether the 'death' state has been entered (terminal flag). */
  const isDead = useRef(false)
  /** rAF handle so we can cancel in-flight animations. */
  const rafRef = useRef<number | null>(null)
  /** Wall-clock start time of the current timed animation. */
  const startTimeRef = useRef<number | null>(null)
  /** Duration of the current timed animation in ms. */
  const durationRef = useRef<number>(0)
  /** Stable ref to the current state so callbacks don't go stale. */
  const stateRef = useRef<AnimationState>('idle')

  /** Cancel any running rAF loop. */
  const cancelAnimation = useCallback(() => {
    if (rafRef.current !== null) {
      cancelAnimationFrame(rafRef.current)
      rafRef.current = null
    }
  }, [])

  /**
   * Start a timed animation for `nextState`.
   * On completion, transitions back to 'idle' (unless it was 'death').
   */
  const startTimedAnimation = useCallback(
    (nextState: Exclude<AnimationState, 'idle'>) => {
      cancelAnimation()
      stateRef.current = nextState
      setState(nextState)
      setProgress(0)

      const duration = DURATIONS[nextState]
      durationRef.current = duration
      startTimeRef.current = null

      const tick = (now: number) => {
        if (startTimeRef.current === null) {
          startTimeRef.current = now
        }
        const elapsed = now - startTimeRef.current
        const p = Math.min(elapsed / duration, 1)
        setProgress(p)

        if (p < 1) {
          rafRef.current = requestAnimationFrame(tick)
        } else {
          rafRef.current = null
          if (nextState !== 'death') {
            stateRef.current = 'idle'
            setState('idle')
            setProgress(0)
          }
        }
      }

      rafRef.current = requestAnimationFrame(tick)
    },
    [cancelAnimation],
  )

  /** Transition to 'attack' state (no-op if already dead). */
  const triggerAttack = useCallback(() => {
    if (isDead.current) return
    startTimedAnimation('attack')
  }, [startTimedAnimation])

  /** Transition to 'damage' state (no-op if already dead). */
  const triggerDamage = useCallback(() => {
    if (isDead.current) return
    startTimedAnimation('damage')
  }, [startTimedAnimation])

  /** Transition to 'death' state (terminal — no further transitions). */
  const triggerDeath = useCallback(() => {
    isDead.current = true
    startTimedAnimation('death')
  }, [startTimedAnimation])

  /** Clean up any pending rAF on unmount. */
  useEffect(() => {
    return () => {
      cancelAnimation()
    }
  }, [cancelAnimation])

  return { state, progress, triggerAttack, triggerDamage, triggerDeath }
}
