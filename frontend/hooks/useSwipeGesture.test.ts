/**
 * @jest-environment jsdom
 *
 * Tests for useSwipeGesture — detects left/right swipe on a touch surface.
 */
import { renderHook, act } from '@testing-library/react'
import useSwipeGesture, { SwipeDirection } from './useSwipeGesture'

/** Build a minimal TouchEvent-like object. */
function makeTouch(clientX: number): Touch {
  return { clientX, clientY: 0, identifier: 0 } as Touch
}

function makeTouchEvent(clientX: number): Partial<TouchEvent> {
  return { changedTouches: [makeTouch(clientX)] as unknown as TouchList }
}

describe('useSwipeGesture', () => {
  it('returns null when no gesture has started', () => {
    const onSwipe = jest.fn()
    const { result } = renderHook(() => useSwipeGesture({ onSwipe, threshold: 50 }))
    expect(result.current.swipeDirection).toBeNull()
  })

  it('detects a right swipe when displacement exceeds threshold', () => {
    const onSwipe = jest.fn()
    const { result } = renderHook(() => useSwipeGesture({ onSwipe, threshold: 50 }))

    act(() => {
      result.current.onTouchStart(makeTouchEvent(100) as TouchEvent)
    })
    act(() => {
      result.current.onTouchEnd(makeTouchEvent(200) as TouchEvent)
    })

    expect(onSwipe).toHaveBeenCalledWith(SwipeDirection.Right)
    expect(result.current.swipeDirection).toBe(SwipeDirection.Right)
  })

  it('detects a left swipe when displacement exceeds threshold', () => {
    const onSwipe = jest.fn()
    const { result } = renderHook(() => useSwipeGesture({ onSwipe, threshold: 50 }))

    act(() => {
      result.current.onTouchStart(makeTouchEvent(200) as TouchEvent)
    })
    act(() => {
      result.current.onTouchEnd(makeTouchEvent(80) as TouchEvent)
    })

    expect(onSwipe).toHaveBeenCalledWith(SwipeDirection.Left)
    expect(result.current.swipeDirection).toBe(SwipeDirection.Left)
  })

  it('does not fire when displacement is below threshold', () => {
    const onSwipe = jest.fn()
    const { result } = renderHook(() => useSwipeGesture({ onSwipe, threshold: 50 }))

    act(() => {
      result.current.onTouchStart(makeTouchEvent(100) as TouchEvent)
    })
    act(() => {
      result.current.onTouchEnd(makeTouchEvent(130) as TouchEvent)
    })

    expect(onSwipe).not.toHaveBeenCalled()
    expect(result.current.swipeDirection).toBeNull()
  })

  it('uses default threshold of 60px when not specified', () => {
    const onSwipe = jest.fn()
    const { result } = renderHook(() => useSwipeGesture({ onSwipe }))

    act(() => {
      result.current.onTouchStart(makeTouchEvent(100) as TouchEvent)
    })
    // 55px — below default 60px threshold
    act(() => {
      result.current.onTouchEnd(makeTouchEvent(155) as TouchEvent)
    })

    expect(onSwipe).not.toHaveBeenCalled()
  })

  it('resets swipeDirection after reset() is called', () => {
    const onSwipe = jest.fn()
    const { result } = renderHook(() => useSwipeGesture({ onSwipe, threshold: 50 }))

    act(() => {
      result.current.onTouchStart(makeTouchEvent(100) as TouchEvent)
      result.current.onTouchEnd(makeTouchEvent(200) as TouchEvent)
    })
    expect(result.current.swipeDirection).toBe(SwipeDirection.Right)

    act(() => {
      result.current.reset()
    })
    expect(result.current.swipeDirection).toBeNull()
  })
})
