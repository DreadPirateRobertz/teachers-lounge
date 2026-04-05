/**
 * useScreenShake — rAF-driven screen-shake effect for critical hits.
 *
 * Returns a CSS transform string that oscillates and decays over 400ms.
 * Maximum displacement is 8px at intensity 1.0.
 */

'use client'

import { useState, useCallback, useRef } from 'react'

/** Return type of the useScreenShake hook. */
export interface UseScreenShakeReturn {
  /**
   * CSS transform string (e.g. `"translate(3px, -2px)"`).
   * Empty string when there is no active shake.
   */
  shakeStyle: string
  /**
   * Fires a shake animation with optional intensity scaling.
   * @param intensity - Multiplier for the max displacement; defaults to 1.0.
   *                    An intensity of 2.0 produces twice the displacement.
   */
  triggerShake: (intensity?: number) => void
}

/** Duration of the shake animation in milliseconds. */
const SHAKE_DURATION_MS = 400

/** Maximum displacement in pixels at intensity 1.0. */
const MAX_DISPLACEMENT_PX = 8

/**
 * Hook that applies a decaying oscillation transform to simulate a screen shake.
 *
 * @returns `{ shakeStyle, triggerShake }` — CSS transform string and trigger fn.
 *
 * @example
 * ```tsx
 * const { shakeStyle, triggerShake } = useScreenShake()
 * // Apply to a wrapper element:
 * <div style={{ transform: shakeStyle }}>{children}</div>
 * // Fire on crit:
 * triggerShake(1.5)
 * ```
 */
export function useScreenShake(): UseScreenShakeReturn {
  const [shakeStyle, setShakeStyle] = useState<string>('')
  const rafRef = useRef<number | null>(null)
  const startTimeRef = useRef<number>(0)
  const intensityRef = useRef<number>(1.0)

  /**
   * Animation loop — samples elapsed time, computes oscillating offsets with
   * exponential decay, then schedules the next frame until the duration elapses.
   * @param timestamp - DOMHighResTimeStamp from requestAnimationFrame.
   */
  const animate = useCallback((timestamp: number) => {
    const elapsed = timestamp - startTimeRef.current
    const progress = Math.min(elapsed / SHAKE_DURATION_MS, 1)

    if (progress >= 1) {
      setShakeStyle('translate(0px, 0px)')
      rafRef.current = null
      return
    }

    // Exponential decay envelope
    const decay = 1 - progress
    const maxDisp = MAX_DISPLACEMENT_PX * intensityRef.current * decay

    // Oscillate at ~30Hz
    const freq = elapsed * 0.03 * Math.PI * 2
    const x = Math.round(Math.cos(freq) * maxDisp)
    const y = Math.round(Math.sin(freq * 1.3) * maxDisp)

    setShakeStyle(`translate(${x}px, ${y}px)`)
    rafRef.current = requestAnimationFrame(animate)
  }, [])

  /**
   * Starts the screen-shake animation.
   * @param intensity - Displacement multiplier; defaults to 1.0.
   */
  const triggerShake = useCallback(
    (intensity: number = 1.0) => {
      if (rafRef.current !== null) {
        cancelAnimationFrame(rafRef.current)
      }
      intensityRef.current = intensity
      startTimeRef.current = performance.now()
      rafRef.current = requestAnimationFrame(animate)
    },
    [animate],
  )

  return { shakeStyle, triggerShake }
}
