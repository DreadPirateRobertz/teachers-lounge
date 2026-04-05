/**
 * @jest-environment jsdom
 *
 * Tests for useMoleculeMorph hook.
 *
 * Verifies initial state, isMorphing transitions, morphProgress range,
 * and completion after 500ms (via manual rAF control).
 */

import { renderHook, act } from '@testing-library/react'
import { useMoleculeMorph } from './useMoleculeMorph'

describe('useMoleculeMorph', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('isMorphing starts false', () => {
    const { result } = renderHook(() => useMoleculeMorph())
    expect(result.current.isMorphing).toBe(false)
  })

  it('triggerMorph() sets isMorphing to true', () => {
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation(() => 1)

    const { result } = renderHook(() => useMoleculeMorph())

    act(() => {
      result.current.triggerMorph()
    })

    expect(result.current.isMorphing).toBe(true)
  })

  it('morphProgress is 0 before trigger', () => {
    const { result } = renderHook(() => useMoleculeMorph())
    expect(result.current.morphProgress).toBe(0)
  })

  it('morphProgress reaches ~1 after 500ms and isMorphing becomes false', () => {
    let rafCb: FrameRequestCallback | null = null
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation((cb) => {
      rafCb = cb
      return 1
    })
    jest.spyOn(global, 'performance', 'get').mockReturnValue({
      now: () => 0,
    } as Performance)

    const { result } = renderHook(() => useMoleculeMorph())

    act(() => {
      result.current.triggerMorph()
    })

    // Fire rAF at 500ms — progress should hit 1 and animation should complete
    act(() => {
      if (rafCb) rafCb(500)
    })

    // After completion, progress resets to 0 and isMorphing is false
    expect(result.current.isMorphing).toBe(false)
    expect(result.current.morphProgress).toBe(0)
  })

  it('morphProgress is between 0 and 1 during animation', () => {
    let rafCb: FrameRequestCallback | null = null
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation((cb) => {
      rafCb = cb
      return 1
    })
    jest.spyOn(global, 'performance', 'get').mockReturnValue({
      now: () => 0,
    } as Performance)

    const { result } = renderHook(() => useMoleculeMorph())

    act(() => {
      result.current.triggerMorph()
    })

    // Fire at midpoint
    act(() => {
      if (rafCb) rafCb(250)
    })

    expect(result.current.morphProgress).toBeGreaterThan(0)
    expect(result.current.morphProgress).toBeLessThanOrEqual(1)
  })
})
