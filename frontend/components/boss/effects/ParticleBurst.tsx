/**
 * ParticleBurst — CSS-only overlay component for answer-feedback particle bursts.
 *
 * Renders particles as absolutely-positioned divs (no canvas) for SSR safety.
 * Exposes `triggerCorrect` and `triggerWrong` via a forwarded ref handle.
 */

'use client'

import React, {
  forwardRef,
  useImperativeHandle,
  type ReactNode,
} from 'react'
import { useParticleBurst, type BurstOrigin } from './useParticleBurst'

/** Imperative handle exposed by ParticleBurst via forwardRef. */
export interface ParticleBurstHandle {
  /**
   * Triggers a correct-answer burst at the given screen-space origin.
   * @param origin - Pixel coordinates of the burst.
   */
  triggerCorrect(origin: BurstOrigin): void
  /**
   * Triggers a wrong-answer burst at the given screen-space origin.
   * @param origin - Pixel coordinates of the burst.
   */
  triggerWrong(origin: BurstOrigin): void
}

/** Props for the ParticleBurst overlay component. */
export interface ParticleBurstProps {
  /** Content rendered underneath the particle overlay. */
  children: ReactNode
}

/**
 * Full-size overlay that renders live particles as CSS divs over its children.
 *
 * Use the imperative handle to fire bursts from parent components:
 * ```tsx
 * const burstRef = useRef<ParticleBurstHandle>(null)
 * burstRef.current?.triggerCorrect({ x: 100, y: 200 })
 * ```
 */
const ParticleBurst = forwardRef<ParticleBurstHandle, ParticleBurstProps>(
  function ParticleBurst({ children }, ref) {
    const { particles, trigger } = useParticleBurst()

    useImperativeHandle(ref, () => ({
      triggerCorrect(origin: BurstOrigin) {
        trigger('correct', origin)
      },
      triggerWrong(origin: BurstOrigin) {
        trigger('wrong', origin)
      },
    }))

    return (
      <div style={{ position: 'relative', width: '100%', height: '100%' }}>
        {children}
        {/* Particle overlay — pointer-events: none so clicks pass through */}
        <div
          aria-hidden="true"
          style={{
            position: 'absolute',
            inset: 0,
            pointerEvents: 'none',
            overflow: 'hidden',
          }}
        >
          {particles.map((p) => {
            const opacity = p.life / p.maxLife
            return (
              <div
                key={p.id}
                style={{
                  position: 'absolute',
                  left: p.x,
                  top: p.y,
                  width: p.size,
                  height: p.size,
                  borderRadius: '50%',
                  backgroundColor: p.color,
                  opacity,
                  transform: 'translate(-50%, -50%)',
                  boxShadow: `0 0 ${p.size * 2}px ${p.color}`,
                }}
              />
            )
          })}
        </div>
      </div>
    )
  },
)

ParticleBurst.displayName = 'ParticleBurst'

export default ParticleBurst
