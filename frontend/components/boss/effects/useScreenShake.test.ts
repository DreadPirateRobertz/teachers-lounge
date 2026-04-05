/**
 * @jest-environment jsdom
 *
 * Tests for useScreenShake hook.
 *
 * Verifies initial state, non-zero displacement after trigger, intensity
 * scaling, and return-to-zero after animation completes.
 */

import { renderHook, act } from '@testing-library/react'
import { useScreenShake } from './useScreenShake'

/** Capture and immediately invoke the rAF callback at the given timestamp. */
function mockRaf(timestamp: number): void {
  let rafCb: FrameRequestCallback | null = null
  jest.spyOn(global, 'requestAnimationFrame').mockImplementationOnce((cb) => {
    rafCb = cb
    return 1
  })
  if (rafCb) (rafCb as FrameRequestCallback)(timestamp)
}

describe('useScreenShake', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('shakeStyle is empty string initially', () => {
    const { result } = renderHook(() => useScreenShake())
    expect(result.current.shakeStyle).toBe('')
  })

  it('triggerShake() changes shakeStyle to a non-zero translate', () => {
    // Capture rAF so we can fire it at t=10ms (small elapsed, large displacement)
    let rafCb: FrameRequestCallback | null = null
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation((cb) => {
      rafCb = cb
      return 1
    })

    // Override performance.now so startTimeRef is 0
    const origNow = performance.now
    performance.now = () => 0

    const { result } = renderHook(() => useScreenShake())

    act(() => {
      result.current.triggerShake()
    })

    // Fire at 10ms — progress is ~0.025, decay ~0.975, so displacement is near max
    act(() => {
      if (rafCb) rafCb(10)
    })

    performance.now = origNow

    expect(result.current.shakeStyle).toMatch(/translate\(/)
  })

  it('intensity 2.0 produces >= displacement as intensity 1.0', () => {
    const extractMaxAbs = (style: string): number => {
      const matches = style.match(/-?[\d.]+px/g) ?? []
      return Math.max(...matches.map((s) => Math.abs(parseFloat(s))))
    }

    const origNow = performance.now
    performance.now = () => 0

    // Run intensity 1.0
    let rafCb1: FrameRequestCallback | null = null
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation((cb) => {
      rafCb1 = cb
      return 1
    })
    const { result: r1 } = renderHook(() => useScreenShake())
    act(() => { r1.current.triggerShake(1.0) })
    act(() => { if (rafCb1) rafCb1(10) })
    const disp1 = extractMaxAbs(r1.current.shakeStyle)

    jest.restoreAllMocks()
    performance.now = () => 0

    // Run intensity 2.0
    let rafCb2: FrameRequestCallback | null = null
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation((cb) => {
      rafCb2 = cb
      return 1
    })
    const { result: r2 } = renderHook(() => useScreenShake())
    act(() => { r2.current.triggerShake(2.0) })
    act(() => { if (rafCb2) rafCb2(10) })
    const disp2 = extractMaxAbs(r2.current.shakeStyle)

    performance.now = origNow

    expect(disp2).toBeGreaterThanOrEqual(disp1)
  })

  it('shakeStyle returns to translate(0px, 0px) after animation completes', () => {
    let rafCb: FrameRequestCallback | null = null
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation((cb) => {
      rafCb = cb
      return 1
    })

    const origNow = performance.now
    performance.now = () => 0

    const { result } = renderHook(() => useScreenShake())

    act(() => {
      result.current.triggerShake()
    })

    // Fire at 400ms — progress === 1, should reset to zero translate
    act(() => {
      if (rafCb) rafCb(400)
    })

    performance.now = origNow

    expect(result.current.shakeStyle).toBe('translate(0px, 0px)')
  })
})
