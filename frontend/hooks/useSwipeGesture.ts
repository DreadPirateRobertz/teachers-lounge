'use client'

/**
 * useSwipeGesture — detects horizontal swipe gestures on touch surfaces.
 *
 * Returns touch event handlers to attach to any element, a stateful
 * `swipeDirection` that records the last detected swipe, and a `reset()`
 * function to clear it.
 *
 * Usage::
 *
 *   const { onTouchStart, onTouchEnd, swipeDirection, reset } = useSwipeGesture({
 *     onSwipe: (dir) => dir === SwipeDirection.Right ? submitAnswer(true) : submitAnswer(false),
 *     threshold: 60,
 *   })
 *
 *   return <div onTouchStart={onTouchStart} onTouchEnd={onTouchEnd}>…</div>
 */
import { useState, useRef, useCallback } from 'react'

/** Horizontal swipe direction. */
export enum SwipeDirection {
  Left = 'left',
  Right = 'right',
}

interface UseSwipeGestureOptions {
  /** Called when a swipe exceeding the threshold is detected. */
  onSwipe: (direction: SwipeDirection) => void
  /**
   * Minimum horizontal pixel displacement to register as a swipe.
   * Defaults to 60px.
   */
  threshold?: number
}

interface UseSwipeGestureResult {
  /** Handler to attach to the element's `onTouchStart`. */
  onTouchStart: (e: TouchEvent | React.TouchEvent) => void
  /** Handler to attach to the element's `onTouchEnd`. */
  onTouchEnd: (e: TouchEvent | React.TouchEvent) => void
  /** The direction of the most recent detected swipe, or null. */
  swipeDirection: SwipeDirection | null
  /** Clears the recorded swipe direction. */
  reset: () => void
}

/**
 * Hook that converts touchstart/touchend events into directional swipe
 * callbacks.
 *
 * @param options - {@link UseSwipeGestureOptions}
 * @returns {@link UseSwipeGestureResult}
 */
export default function useSwipeGesture({
  onSwipe,
  threshold = 60,
}: UseSwipeGestureOptions): UseSwipeGestureResult {
  const startX = useRef<number | null>(null)
  const [swipeDirection, setSwipeDirection] = useState<SwipeDirection | null>(null)

  const onTouchStart = useCallback((e: TouchEvent | React.TouchEvent) => {
    const touch = e.changedTouches[0]
    startX.current = touch.clientX
  }, [])

  const onTouchEnd = useCallback(
    (e: TouchEvent | React.TouchEvent) => {
      if (startX.current === null) return
      const touch = e.changedTouches[0]
      const delta = touch.clientX - startX.current
      startX.current = null

      if (Math.abs(delta) < threshold) return

      const direction = delta > 0 ? SwipeDirection.Right : SwipeDirection.Left
      setSwipeDirection(direction)
      onSwipe(direction)
    },
    [onSwipe, threshold],
  )

  const reset = useCallback(() => {
    setSwipeDirection(null)
  }, [])

  return { onTouchStart, onTouchEnd, swipeDirection, reset }
}
