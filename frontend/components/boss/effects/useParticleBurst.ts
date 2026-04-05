/**
 * useParticleBurst — rAF-ticked particle burst effect for correct/wrong answers.
 *
 * Spawns neon-green/blue particles on correct answers and neon-pink particles
 * on wrong answers. Dead particles are auto-pruned each frame.
 */

'use client'

import { useState, useCallback, useRef } from 'react'

/** The type of answer event that triggered the burst. */
export type BurstType = 'correct' | 'wrong'

/** 2-D origin coordinates for a particle burst. */
export interface BurstOrigin {
  /** Horizontal pixel position. */
  x: number
  /** Vertical pixel position. */
  y: number
}

/** A single particle in the burst simulation. */
export interface Particle {
  /** Unique identifier for React key prop. */
  id: number
  /** Current horizontal position in pixels. */
  x: number
  /** Current vertical position in pixels. */
  y: number
  /** Horizontal velocity in pixels per frame. */
  vx: number
  /** Vertical velocity in pixels per frame. */
  vy: number
  /** Remaining lifetime in frames. */
  life: number
  /** Initial lifetime in frames (used to compute opacity). */
  maxLife: number
  /** CSS hex color string. */
  color: string
  /** Diameter of the particle in pixels. */
  size: number
}

/** Return type of the useParticleBurst hook. */
export interface UseParticleBurstReturn {
  /** Currently live particles — updated each animation frame. */
  particles: Particle[]
  /**
   * Spawns a burst of particles at the given origin.
   * @param type - 'correct' spawns 20 neon-green/blue particles;
   *               'wrong' spawns 12 neon-pink particles that fall with gravity.
   * @param origin - Pixel coordinates of the burst origin.
   */
  trigger: (type: BurstType, origin: BurstOrigin) => void
}

/** Neon color tokens. */
const NEON_GREEN = '#00ff88'
const NEON_BLUE = '#00aaff'
const NEON_PINK = '#ff00aa'

/** Number of particles spawned per burst type. */
const CORRECT_COUNT = 20
const WRONG_COUNT = 12

/** Gravity applied to wrong-answer particles (pixels per frame²). */
const GRAVITY = 0.4

let nextId = 0

/**
 * Generates a random float in [min, max).
 * @param min - Lower bound (inclusive).
 * @param max - Upper bound (exclusive).
 */
function rand(min: number, max: number): number {
  return min + Math.random() * (max - min)
}

/**
 * Builds a particle array for a burst event.
 * @param type - The answer result that triggered the burst.
 * @param origin - Screen-space coordinates for the burst origin.
 */
function buildParticles(type: BurstType, origin: BurstOrigin): Particle[] {
  const count = type === 'correct' ? CORRECT_COUNT : WRONG_COUNT
  const particles: Particle[] = []

  for (let i = 0; i < count; i++) {
    const angle = (Math.PI * 2 * i) / count + rand(-0.3, 0.3)
    const speed = type === 'correct' ? rand(3, 7) : rand(2, 5)

    const vx = Math.cos(angle) * speed
    const vy = type === 'correct' ? Math.sin(angle) * speed : rand(-4, -1)

    let color: string
    if (type === 'correct') {
      color = i % 2 === 0 ? NEON_GREEN : NEON_BLUE
    } else {
      color = NEON_PINK
    }

    const maxLife = Math.round(rand(30, 60))
    particles.push({
      id: nextId++,
      x: origin.x,
      y: origin.y,
      vx,
      vy,
      life: maxLife,
      maxLife,
      color,
      size: Math.round(rand(4, 10)),
    })
  }

  return particles
}

/**
 * Hook that manages a rAF-driven particle simulation for answer feedback bursts.
 *
 * @returns `{ particles, trigger }` — live particle array and trigger function.
 *
 * @example
 * ```tsx
 * const { particles, trigger } = useParticleBurst()
 * // on correct answer:
 * trigger('correct', { x: 200, y: 300 })
 * ```
 */
export function useParticleBurst(): UseParticleBurstReturn {
  const [particles, setParticles] = useState<Particle[]>([])
  const rafRef = useRef<number | null>(null)
  const particlesRef = useRef<Particle[]>([])

  /** Advances the simulation by one frame and schedules the next tick. */
  const tick = useCallback(() => {
    particlesRef.current = particlesRef.current
      .map((p) => ({
        ...p,
        x: p.x + p.vx,
        y: p.y + p.vy,
        vy: p.vy + GRAVITY,
        life: p.life - 1,
      }))
      .filter((p) => p.life > 0)

    setParticles([...particlesRef.current])

    if (particlesRef.current.length > 0) {
      rafRef.current = requestAnimationFrame(tick)
    } else {
      rafRef.current = null
    }
  }, [])

  /**
   * Spawns a new burst of particles and starts the animation loop if needed.
   * @param type - Answer result type.
   * @param origin - Pixel origin of the burst.
   */
  const trigger = useCallback(
    (type: BurstType, origin: BurstOrigin) => {
      const newParticles = buildParticles(type, origin)
      particlesRef.current = [...particlesRef.current, ...newParticles]
      setParticles([...particlesRef.current])

      if (rafRef.current === null) {
        rafRef.current = requestAnimationFrame(tick)
      }
    },
    [tick],
  )

  return { particles, trigger }
}
