/**
 * @jest-environment jsdom
 *
 * Tests for useBossAnimation state machine.
 * Uses fake timers and mocked requestAnimationFrame.
 */

import { act, renderHook } from '@testing-library/react'
import { useBossAnimation } from './useBossAnimation'

// ---------------------------------------------------------------------------
// rAF mock — synchronous tick driven by jest.runAllTimers
// ---------------------------------------------------------------------------

let rafCallbacks: ((time: number) => void)[] = []
let mockNow = 0

beforeEach(() => {
  rafCallbacks = []
  mockNow = 0

  jest.spyOn(globalThis, 'requestAnimationFrame').mockImplementation((cb) => {
    rafCallbacks.push(cb)
    return rafCallbacks.length
  })

  jest.spyOn(globalThis, 'cancelAnimationFrame').mockImplementation((id) => {
    rafCallbacks[id - 1] = () => undefined
  })
})

afterEach(() => {
  jest.restoreAllMocks()
})

/** Flush one rAF frame with the given time delta (ms). */
function flushFrame(deltaMsAdvance = 16): void {
  mockNow += deltaMsAdvance
  const callbacks = [...rafCallbacks]
  rafCallbacks = []
  for (const cb of callbacks) {
    cb(mockNow)
  }
}

/** Flush frames until all scheduled callbacks are drained. */
function flushAllFrames(deltaMs = 16, maxFrames = 200): void {
  let i = 0
  while (rafCallbacks.length > 0 && i++ < maxFrames) {
    flushFrame(deltaMs)
  }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useBossAnimation', () => {
  it('starts in idle state', () => {
    const { result } = renderHook(() => useBossAnimation())
    expect(result.current.state).toBe('idle')
  })

  it('starts with progress 0', () => {
    const { result } = renderHook(() => useBossAnimation())
    expect(result.current.progress).toBe(0)
  })

  it('triggerAttack transitions to attack state', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerAttack()
    })

    expect(result.current.state).toBe('attack')
  })

  it('triggerDamage transitions to damage state', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerDamage()
    })

    expect(result.current.state).toBe('damage')
  })

  it('triggerDeath transitions to death state', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerDeath()
    })

    expect(result.current.state).toBe('death')
  })

  it('progress starts at 0 immediately after triggerAttack', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerAttack()
    })

    // Before any rAF ticks, progress should be 0
    expect(result.current.progress).toBe(0)
  })

  it('progress starts at 0 immediately after triggerDamage', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerDamage()
    })

    expect(result.current.progress).toBe(0)
  })

  it('progress advances above 0 after rAF frames during attack', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerAttack()
    })

    act(() => {
      // First frame initialises startTimeRef; second frame produces real elapsed.
      flushFrame(100) // sets startTime
      flushFrame(100) // elapsed = 100ms → progress = 100/600 > 0
    })

    expect(result.current.progress).toBeGreaterThan(0)
    expect(result.current.progress).toBeLessThanOrEqual(1)
  })

  it('attack animation returns to idle after completing', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerAttack()
    })

    act(() => {
      // Flush well past 600ms duration
      flushAllFrames(100)
    })

    expect(result.current.state).toBe('idle')
  })

  it('damage animation returns to idle after completing', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerDamage()
    })

    act(() => {
      flushAllFrames(100)
    })

    expect(result.current.state).toBe('idle')
  })

  it('death is terminal — triggerAttack is a no-op after death', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerDeath()
    })

    expect(result.current.state).toBe('death')

    act(() => {
      result.current.triggerAttack()
    })

    expect(result.current.state).toBe('death')
  })

  it('death is terminal — triggerDamage is a no-op after death', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerDeath()
    })

    act(() => {
      result.current.triggerDamage()
    })

    expect(result.current.state).toBe('death')
  })

  it('death is terminal — triggerDeath again is a no-op (state stays death)', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerDeath()
    })

    act(() => {
      flushAllFrames(100)
    })

    // Should remain in death, not transition to idle
    expect(result.current.state).toBe('death')
  })

  it('death animation does not transition to idle after completing', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerDeath()
    })

    act(() => {
      // Flush well past 1200ms duration
      flushAllFrames(200)
    })

    expect(result.current.state).toBe('death')
  })

  it('triggerAttack from death is no-op', () => {
    const { result } = renderHook(() => useBossAnimation())

    act(() => {
      result.current.triggerDeath()
      result.current.triggerAttack()
    })

    expect(result.current.state).toBe('death')
  })
})
