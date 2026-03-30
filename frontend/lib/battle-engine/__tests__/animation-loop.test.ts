import { createAnimationLoop } from '../animation-loop'

// Mock requestAnimationFrame/cancelAnimationFrame
let rafCallbacks: Array<(time: number) => void> = []
let rafId = 0

beforeEach(() => {
  rafCallbacks = []
  rafId = 0
  globalThis.requestAnimationFrame = (cb: FrameRequestCallback) => {
    rafCallbacks.push(cb as (time: number) => void)
    return ++rafId
  }
  globalThis.cancelAnimationFrame = () => {
    rafCallbacks = []
  }
  jest.spyOn(performance, 'now').mockReturnValue(0)
})

function flushFrame(time: number) {
  const cbs = [...rafCallbacks]
  rafCallbacks = []
  for (const cb of cbs) cb(time)
}

describe('createAnimationLoop', () => {
  it('calls registered callbacks with delta time', () => {
    const loop = createAnimationLoop()
    const fn = jest.fn()

    loop.add('test', fn)
    // First frame establishes baseline
    flushFrame(0)
    // Second frame provides delta
    flushFrame(16.67)

    expect(fn).toHaveBeenCalledTimes(2)
    const dt = fn.mock.calls[1][0]
    expect(dt).toBeCloseTo(0.01667, 3)

    loop.dispose()
  })

  it('removes callbacks by id', () => {
    const loop = createAnimationLoop()
    const fn = jest.fn()

    loop.add('a', fn)
    loop.remove('a')
    // Loop should auto-stop with no callbacks

    expect(loop.running).toBe(false)
    loop.dispose()
  })

  it('caps delta time to prevent physics explosions', () => {
    const loop = createAnimationLoop()
    const fn = jest.fn()

    loop.add('test', fn)
    flushFrame(0)
    // Simulate 500ms gap (tab refocus)
    flushFrame(500)

    const dt = fn.mock.calls[1][0]
    expect(dt).toBeLessThanOrEqual(0.05) // MAX_DT = 1/20

    loop.dispose()
  })

  it('prevents duplicate ids', () => {
    const loop = createAnimationLoop()
    const fn1 = jest.fn()
    const fn2 = jest.fn()

    loop.add('same', fn1)
    loop.add('same', fn2)
    flushFrame(0)

    expect(fn1).not.toHaveBeenCalled()
    expect(fn2).toHaveBeenCalledTimes(1)

    loop.dispose()
  })

  it('auto-starts on first add and auto-stops on last remove', () => {
    const loop = createAnimationLoop()
    expect(loop.running).toBe(false)

    loop.add('a', () => {})
    expect(loop.running).toBe(true)

    loop.remove('a')
    expect(loop.running).toBe(false)

    loop.dispose()
  })
})
