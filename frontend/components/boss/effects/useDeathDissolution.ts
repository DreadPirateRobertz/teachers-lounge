/**
 * useDeathDissolution — 1200ms boss death dissolution animation.
 *
 * Produces a dissolveProgress value (0→1) designed to drive Three.js material
 * opacity fades and particle scatter on boss defeat.
 */

'use client'

import { useState, useCallback, useRef } from 'react'

/** Return type of the useDeathDissolution hook. */
export interface UseDeathDissolutionReturn {
  /**
   * Dissolution progress from 0 (intact) to 1 (fully dissolved).
   * Stays at 1 after the animation completes.
   */
  dissolveProgress: number
  /** True while the dissolution animation is running. */
  isDissolving: boolean
  /**
   * Starts the 1200ms dissolution animation.
   * Has no effect if dissolution is already in progress.
   */
  triggerDissolve: () => void
}

/** Duration of the dissolution animation in milliseconds. */
const DISSOLVE_DURATION_MS = 1200

/**
 * Hook that animates a 0→1 progress value over 1200ms for boss death VFX.
 *
 * @returns `{ dissolveProgress, isDissolving, triggerDissolve }`.
 *
 * @example
 * ```tsx
 * const { dissolveProgress, triggerDissolve } = useDeathDissolution()
 * // Trigger when boss HP reaches 0:
 * triggerDissolve()
 * // Drive Three.js material: material.opacity = 1 - dissolveProgress
 * ```
 */
export function useDeathDissolution(): UseDeathDissolutionReturn {
  const [dissolveProgress, setDissolveProgress] = useState<number>(0)
  const [isDissolving, setIsDissolving] = useState<boolean>(false)
  const rafRef = useRef<number | null>(null)
  const startTimeRef = useRef<number>(0)

  /**
   * Animation loop — advances dissolveProgress from 0 to 1 over DISSOLVE_DURATION_MS.
   * @param timestamp - DOMHighResTimeStamp from requestAnimationFrame.
   */
  const animate = useCallback((timestamp: number) => {
    const elapsed = timestamp - startTimeRef.current
    const progress = Math.min(elapsed / DISSOLVE_DURATION_MS, 1)

    setDissolveProgress(progress)

    if (progress < 1) {
      rafRef.current = requestAnimationFrame(animate)
    } else {
      setIsDissolving(false)
      rafRef.current = null
    }
  }, [])

  /**
   * Starts the dissolution animation. No-op if already dissolving.
   */
  const triggerDissolve = useCallback(() => {
    if (isDissolving) return
    startTimeRef.current = performance.now()
    setDissolveProgress(0)
    setIsDissolving(true)
    rafRef.current = requestAnimationFrame(animate)
  }, [isDissolving, animate])

  return { dissolveProgress, isDissolving, triggerDissolve }
}
