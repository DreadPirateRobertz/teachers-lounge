/**
 * useMoleculeMorph — 500ms morph animation for boss geometry distortion on damage.
 *
 * Drives a morphProgress value from 0 to 1 over the animation window.
 * Intended for use with BossCanvas to deform the boss mesh on a damage hit.
 */

'use client'

import { useState, useCallback, useRef } from 'react'

/** Return type of the useMoleculeMorph hook. */
export interface UseMoleculeMorphReturn {
  /**
   * Animation progress from 0 (start) to 1 (complete).
   * Resets to 0 after the animation finishes.
   */
  morphProgress: number
  /** True while the morph animation is running. */
  isMorphing: boolean
  /**
   * Starts the 500ms morph animation.
   * Calling while already morphing restarts the animation.
   */
  triggerMorph: () => void
}

/** Duration of the morph animation in milliseconds. */
const MORPH_DURATION_MS = 500

/**
 * Hook that produces a 0→1 progress value over a 500ms animation window.
 *
 * @returns `{ morphProgress, isMorphing, triggerMorph }`.
 *
 * @example
 * ```tsx
 * const { morphProgress, triggerMorph } = useMoleculeMorph()
 * // Trigger on damage:
 * triggerMorph()
 * // Pass morphProgress to the Three.js canvas for geometry distortion.
 * ```
 */
export function useMoleculeMorph(): UseMoleculeMorphReturn {
  const [morphProgress, setMorphProgress] = useState<number>(0)
  const [isMorphing, setIsMorphing] = useState<boolean>(false)
  const rafRef = useRef<number | null>(null)
  const startTimeRef = useRef<number>(0)

  /**
   * Animation loop — advances morphProgress from 0 to 1 over MORPH_DURATION_MS.
   * @param timestamp - DOMHighResTimeStamp from requestAnimationFrame.
   */
  const animate = useCallback((timestamp: number) => {
    const elapsed = timestamp - startTimeRef.current
    const progress = Math.min(elapsed / MORPH_DURATION_MS, 1)

    setMorphProgress(progress)

    if (progress < 1) {
      rafRef.current = requestAnimationFrame(animate)
    } else {
      setIsMorphing(false)
      setMorphProgress(0)
      rafRef.current = null
    }
  }, [])

  /**
   * Starts (or restarts) the morph animation.
   */
  const triggerMorph = useCallback(() => {
    if (rafRef.current !== null) {
      cancelAnimationFrame(rafRef.current)
    }
    startTimeRef.current = performance.now()
    setMorphProgress(0)
    setIsMorphing(true)
    rafRef.current = requestAnimationFrame(animate)
  }, [animate])

  return { morphProgress, isMorphing, triggerMorph }
}
