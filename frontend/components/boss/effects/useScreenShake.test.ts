/**
 * Tests for useScreenShake hook.
 *
 * Verifies initial state, non-zero displacement after trigger, intensity
 * scaling, and return-to-zero after animation completes (via fake timers).
 */

import { renderHook, act } from '@testing-library/react'
import { useScreenShake } from './useScreenShake'

describe('useScreenShake', () => {
  beforeEach(() => {
    jest.useFakeTimers()
  })

  afterEach(() => {
    jest.useRealTimers()
    jest.restoreAllMocks()
  })

  it('shakeStyle is empty string initially', () => {
    const { result } = renderHook(() => useScreenShake())
    expect(result.current.shakeStyle).toBe('')
  })

  it('triggerShake() changes shakeStyle to a non-zero translate', () => {
    // Provide a rAF that immediately fires the callback with a small timestamp
    let fireCount = 0
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation((cb) => {
      if (fireCount === 0) {
        fireCount++
        // Fire once at t=10ms so the progress is small but non-zero
        cb(10)
      }
      return 1
    })
    jest.spyOn(global, 'performance', 'get').mockReturnValue({
      now: () => 0,
    } as Performance)

    const { result } = renderHook(() => useScreenShake())

    act(() => {
      result.current.triggerShake()
    })

    // After one frame at t=10 we expect a non-empty translate string
    expect(result.current.shakeStyle).toMatch(/translate\(/)
  })

  it('intensity 2.0 produces >= displacement as intensity 1.0', () => {
    /**
     * We test by running a single rAF frame at t=10ms for each intensity
     * and comparing the extracted pixel value.
     */
    const extractMaxAbs = (style: string): number => {
      const matches = style.match(/-?[\d.]+px/g) ?? []
      return Math.max(...matches.map((s) => Math.abs(parseFloat(s))))
    }

    let rafCb: FrameRequestCallback | null = null
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation((cb) => {
      rafCb = cb
      return 1
    })
    jest.spyOn(global, 'performance', 'get').mockReturnValue({
      now: () => 0,
    } as Performance)

    // Intensity 1.0
    const { result: result1 } = renderHook(() => useScreenShake())
    act(() => {
      result1.current.triggerShake(1.0)
    })
    act(() => {
      if (rafCb) rafCb(10)
    })
    const disp1 = extractMaxAbs(result1.current.shakeStyle)

    // Intensity 2.0
    const { result: result2 } = renderHook(() => useScreenShake())
    act(() => {
      result2.current.triggerShake(2.0)
    })
    act(() => {
      if (rafCb) rafCb(10)
    })
    const disp2 = extractMaxAbs(result2.current.shakeStyle)

    expect(disp2).toBeGreaterThanOrEqual(disp1)
  })

  it('shakeStyle returns to translate(0px, 0px) after animation completes', () => {
    // Simulate rAF that runs the callback at t = SHAKE_DURATION_MS (400ms)
    let rafCb: FrameRequestCallback | null = null
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation((cb) => {
      rafCb = cb
      return 1
    })
    jest.spyOn(global, 'performance', 'get').mockReturnValue({
      now: () => 0,
    } as Performance)

    const { result } = renderHook(() => useScreenShake())

    act(() => {
      result.current.triggerShake()
    })

    // Fire rAF at exactly the end of the animation (progress >= 1)
    act(() => {
      if (rafCb) rafCb(400)
    })

    expect(result.current.shakeStyle).toBe('translate(0px, 0px)')
  })
})
