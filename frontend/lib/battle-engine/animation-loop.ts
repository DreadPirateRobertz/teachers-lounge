import type { FrameCallback } from './types'

const MAX_DT = 1 / 20 // Cap delta to 50ms to prevent physics explosions on tab-refocus

/**
 * Manages the requestAnimationFrame loop with fixed-priority callback ordering.
 * Callbacks run in registration order each frame. The loop auto-pauses when
 * no callbacks are registered and auto-stops on dispose.
 */
export function createAnimationLoop() {
  const callbacks: { id: string; fn: FrameCallback }[] = []
  let rafId: number | null = null
  let lastTime = 0
  let running = false

  function tick(now: number) {
    if (!running) return
    rafId = requestAnimationFrame(tick)

    const dt = Math.min((now - lastTime) / 1000, MAX_DT)
    lastTime = now

    for (const cb of callbacks) {
      cb.fn(dt)
    }
  }

  function start() {
    if (running) return
    running = true
    lastTime = performance.now()
    rafId = requestAnimationFrame(tick)
  }

  function stop() {
    running = false
    if (rafId !== null) {
      cancelAnimationFrame(rafId)
      rafId = null
    }
  }

  function add(id: string, fn: FrameCallback) {
    // Prevent duplicate ids
    remove(id)
    callbacks.push({ id, fn })
    if (callbacks.length === 1 && !running) start()
  }

  function remove(id: string) {
    const idx = callbacks.findIndex((cb) => cb.id === id)
    if (idx !== -1) callbacks.splice(idx, 1)
    if (callbacks.length === 0 && running) stop()
  }

  function dispose() {
    stop()
    callbacks.length = 0
  }

  return { start, stop, add, remove, dispose, get running() { return running } }
}

export type AnimationLoop = ReturnType<typeof createAnimationLoop>
