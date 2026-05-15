'use client'

/**
 * VictoryParticles.tsx (hq-4lun)
 *
 * Lightweight canvas-based confetti burst rendered on top of LootReveal when
 * the player clears a boss. Self-contained — no third-party particle library.
 *
 * Honours `prefers-reduced-motion`: if the user has reduced-motion enabled,
 * the component renders nothing at all (no canvas, no animation loop).
 */

import { useEffect, useRef } from 'react'

/** Single confetti shred. Mutated in place each frame. */
interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  size: number
  rotation: number
  rotationSpeed: number
  colour: string
}

/** Neon palette matched to the LootReveal panel border / content colours. */
const COLOURS = [
  '#ffd700', // neon-gold
  '#22d3ee', // neon-blue
  '#f472b6', // neon-pink
  '#4ade80', // neon-green
  '#a78bfa', // neon-purple
]

/** Total run-time for the burst in milliseconds. */
const DURATION_MS = 2200

/** Number of confetti pieces per burst. */
const PARTICLE_COUNT = 80

/** Gravity in px/s² applied to each particle's vy each frame. */
const GRAVITY = 900

export interface VictoryParticlesProps {
  /**
   * When true, the burst is mounted and animation begins. Toggling false
   * (or unmounting the component) stops the loop and clears the canvas.
   */
  active: boolean
}

/**
 * Returns true if the user has requested reduced motion. SSR-safe.
 */
function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return false
  }
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * VictoryParticles renders a one-shot confetti burst on a fixed full-screen
 * canvas. The canvas is purely decorative (`aria-hidden`) and does not block
 * pointer events.
 */
export default function VictoryParticles({ active }: VictoryParticlesProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const reduced = prefersReducedMotion()

  useEffect(() => {
    if (!active || reduced) return
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const dpr = typeof window !== 'undefined' ? window.devicePixelRatio || 1 : 1
    const width = window.innerWidth
    const height = window.innerHeight
    canvas.width = Math.floor(width * dpr)
    canvas.height = Math.floor(height * dpr)
    canvas.style.width = `${width}px`
    canvas.style.height = `${height}px`
    ctx.scale(dpr, dpr)

    const originX = width / 2
    const originY = height / 2
    const particles: Particle[] = Array.from({ length: PARTICLE_COUNT }, () => {
      const angle = Math.random() * Math.PI * 2
      const speed = 250 + Math.random() * 350
      return {
        x: originX,
        y: originY,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed - 200,
        size: 4 + Math.random() * 4,
        rotation: Math.random() * Math.PI * 2,
        rotationSpeed: (Math.random() - 0.5) * 8,
        colour: COLOURS[Math.floor(Math.random() * COLOURS.length)],
      }
    })

    let rafId = 0
    let lastT = performance.now()
    const start = lastT

    const tick = (now: number) => {
      const dt = Math.min(0.05, (now - lastT) / 1000)
      lastT = now
      const elapsed = now - start
      const lifeFrac = Math.min(1, elapsed / DURATION_MS)
      const alpha = 1 - lifeFrac

      ctx.clearRect(0, 0, width, height)

      for (const p of particles) {
        p.vy += GRAVITY * dt
        p.x += p.vx * dt
        p.y += p.vy * dt
        p.rotation += p.rotationSpeed * dt

        ctx.save()
        ctx.globalAlpha = alpha
        ctx.translate(p.x, p.y)
        ctx.rotate(p.rotation)
        ctx.fillStyle = p.colour
        ctx.fillRect(-p.size / 2, -p.size / 2, p.size, p.size * 0.5)
        ctx.restore()
      }

      if (elapsed < DURATION_MS) {
        rafId = requestAnimationFrame(tick)
      } else {
        ctx.clearRect(0, 0, width, height)
      }
    }

    rafId = requestAnimationFrame(tick)
    return () => {
      cancelAnimationFrame(rafId)
      ctx.clearRect(0, 0, width, height)
    }
  }, [active, reduced])

  if (!active || reduced) return null

  return (
    <canvas
      ref={canvasRef}
      data-testid="victory-particles"
      aria-hidden="true"
      className="fixed inset-0 pointer-events-none z-40"
    />
  )
}
