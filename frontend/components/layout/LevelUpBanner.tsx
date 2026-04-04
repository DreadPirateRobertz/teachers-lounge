'use client'

import { useEffect, useState } from 'react'

interface Props {
  /** New level just reached */
  newLevel: number
  /** Called when the animation finishes and banner unmounts */
  onDismiss?: () => void
}

export default function LevelUpBanner({ newLevel, onDismiss }: Props) {
  const [visible, setVisible] = useState(true)

  useEffect(() => {
    const t = setTimeout(() => {
      setVisible(false)
      onDismiss?.()
    }, 3000)
    return () => clearTimeout(t)
  }, [onDismiss])

  if (!visible) return null

  return (
    <div
      className="fixed inset-x-0 bottom-16 flex justify-center items-end z-50 pointer-events-none"
      aria-live="polite"
    >
      <div className="animate-level-up flex flex-col items-center gap-1 px-8 py-4 rounded-2xl border border-neon-gold/60 bg-bg-deep/90 backdrop-blur-sm"
        style={{ boxShadow: '0 0 24px #ffdc0044, 0 0 60px #ffdc0022, inset 0 0 20px #ffdc0011' }}
      >
        {/* Stars row */}
        <div className="flex gap-2 text-lg mb-1">
          {['⭐', '✨', '⭐'].map((s, i) => (
            <span
              key={i}
              style={{ animationDelay: `${i * 0.12}s` }}
              className="animate-bounce-slow inline-block"
            >
              {s}
            </span>
          ))}
        </div>

        <div className="font-mono text-[11px] uppercase tracking-[0.25em] text-neon-gold/70 text-glow-gold">
          Level Up!
        </div>

        <div
          className="animate-level-num-pop font-mono text-4xl font-black text-neon-gold text-glow-gold leading-none"
        >
          {newLevel}
        </div>

        <div className="font-mono text-xs text-text-dim mt-0.5">
          You reached <span className="text-neon-gold">Level {newLevel}</span>
        </div>
      </div>
    </div>
  )
}
