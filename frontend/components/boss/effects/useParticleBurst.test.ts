/**
 * Tests for useParticleBurst hook.
 *
 * Verifies particle counts, colors, life values, and dead-particle pruning.
 */

import { renderHook, act } from '@testing-library/react'
import { useParticleBurst } from './useParticleBurst'

const ORIGIN = { x: 100, y: 200 }

describe('useParticleBurst', () => {
  beforeEach(() => {
    // Provide a minimal rAF implementation so the hook can schedule ticks
    let id = 0
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation((cb) => {
      // Don't auto-advance; tests control frame execution manually
      id++
      return id
    })
    jest.spyOn(global, 'cancelAnimationFrame').mockImplementation(() => {})
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('trigger("correct", origin) spawns 20 particles', () => {
    const { result } = renderHook(() => useParticleBurst())

    act(() => {
      result.current.trigger('correct', ORIGIN)
    })

    expect(result.current.particles).toHaveLength(20)
  })

  it('trigger("wrong", origin) spawns 12 particles', () => {
    const { result } = renderHook(() => useParticleBurst())

    act(() => {
      result.current.trigger('wrong', ORIGIN)
    })

    expect(result.current.particles).toHaveLength(12)
  })

  it('correct particles use neon-green and neon-blue colors', () => {
    const { result } = renderHook(() => useParticleBurst())

    act(() => {
      result.current.trigger('correct', ORIGIN)
    })

    const colors = result.current.particles.map((p) => p.color)
    expect(colors).toContain('#00ff88')
    expect(colors).toContain('#00aaff')
    colors.forEach((c) => {
      expect(['#00ff88', '#00aaff']).toContain(c)
    })
  })

  it('wrong particles use neon-pink color only', () => {
    const { result } = renderHook(() => useParticleBurst())

    act(() => {
      result.current.trigger('wrong', ORIGIN)
    })

    result.current.particles.forEach((p) => {
      expect(p.color).toBe('#ff00aa')
    })
  })

  it('all particles have positive maxLife', () => {
    const { result } = renderHook(() => useParticleBurst())

    act(() => {
      result.current.trigger('correct', ORIGIN)
    })

    result.current.particles.forEach((p) => {
      expect(p.maxLife).toBeGreaterThan(0)
    })
  })

  it('dead particles (life <= 0) are absent from the returned array', () => {
    // Capture the rAF callback so we can advance it manually
    let savedCb: FrameRequestCallback | null = null
    ;(global.requestAnimationFrame as jest.Mock).mockImplementation(
      (cb: FrameRequestCallback) => {
        savedCb = cb
        return 1
      },
    )

    const { result } = renderHook(() => useParticleBurst())

    act(() => {
      result.current.trigger('wrong', ORIGIN)
    })

    // Drain all life by running enough frames (max maxLife is 60)
    act(() => {
      for (let i = 0; i < 65; i++) {
        if (savedCb) {
          savedCb(performance.now())
        }
      }
    })

    expect(result.current.particles).toHaveLength(0)
  })
})
