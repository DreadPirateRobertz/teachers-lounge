/**
 * @jest-environment jsdom
 *
 * Tests for useDeathDissolution hook.
 *
 * Verifies initial state, isDissolving transitions, and dissolveProgress
 * progression after 1200ms.
 */

import { renderHook, act } from '@testing-library/react'
import { useDeathDissolution } from './useDeathDissolution'

describe('useDeathDissolution', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('isDissolving starts false', () => {
    const { result } = renderHook(() => useDeathDissolution())
    expect(result.current.isDissolving).toBe(false)
  })

  it('triggerDissolve() sets isDissolving to true', () => {
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation(() => 1)

    const { result } = renderHook(() => useDeathDissolution())

    act(() => {
      result.current.triggerDissolve()
    })

    expect(result.current.isDissolving).toBe(true)
  })

  it('dissolveProgress is 0 before trigger', () => {
    const { result } = renderHook(() => useDeathDissolution())
    expect(result.current.dissolveProgress).toBe(0)
  })

  it('dissolveProgress reaches ~1 after 1200ms', () => {
    let rafCb: FrameRequestCallback | null = null
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation((cb) => {
      rafCb = cb
      return 1
    })
    jest.spyOn(global, 'performance', 'get').mockReturnValue({
      now: () => 0,
    } as Performance)

    const { result } = renderHook(() => useDeathDissolution())

    act(() => {
      result.current.triggerDissolve()
    })

    // Fire rAF at 1200ms — progress should reach 1 and animation complete
    act(() => {
      if (rafCb) rafCb(1200)
    })

    expect(result.current.dissolveProgress).toBeCloseTo(1, 5)
    expect(result.current.isDissolving).toBe(false)
  })

  it('dissolveProgress is between 0 and 1 at the midpoint', () => {
    let rafCb: FrameRequestCallback | null = null
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation((cb) => {
      rafCb = cb
      return 1
    })
    jest.spyOn(global, 'performance', 'get').mockReturnValue({
      now: () => 0,
    } as Performance)

    const { result } = renderHook(() => useDeathDissolution())

    act(() => {
      result.current.triggerDissolve()
    })

    act(() => {
      if (rafCb) rafCb(600)
    })

    expect(result.current.dissolveProgress).toBeGreaterThan(0)
    expect(result.current.dissolveProgress).toBeLessThan(1)
  })
})
