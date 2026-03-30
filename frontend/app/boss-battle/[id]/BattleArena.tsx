'use client'

import { useCallback, useState } from 'react'
import { BattleCanvas } from '@/components/boss-battle/BattleCanvas'
import type { BattleEngine } from '@/lib/battle-engine'

interface BattleArenaProps {
  chapterId: string
}

/**
 * Client-side battle arena. Initializes the engine and renders the
 * Three.js canvas with a loading overlay. Downstream beads (tl-08c, tl-2l7)
 * will add boss definitions and the battle state machine here.
 */
export function BattleArena({ chapterId }: BattleArenaProps) {
  const [loading, setLoading] = useState(true)

  const handleReady = useCallback(
    (engine: BattleEngine) => {
      // Engine is initialized — start the render loop.
      // Boss loading and battle logic will be wired by tl-08c and tl-2l7.
      engine.start()
      setLoading(false)
    },
    [],
  )

  return (
    <div className="relative w-full h-screen bg-bg-deep overflow-hidden">
      <BattleCanvas onReady={handleReady} />

      {/* HUD overlay — chapter label */}
      <div className="absolute top-4 left-4 z-10">
        <p className="font-mono text-xs text-text-dim">
          Chapter {chapterId}
        </p>
        <h1 className="font-mono text-lg font-bold text-neon-pink text-glow-pink">
          Boss Battle
        </h1>
      </div>

      {/* Loading overlay */}
      {loading && (
        <div className="absolute inset-0 z-20 flex flex-col items-center justify-center bg-bg-deep/90">
          <div className="text-6xl animate-pulse-slow mb-4">⚗️</div>
          <p className="font-mono text-sm text-neon-blue animate-glow-pulse">
            Initializing battle engine...
          </p>
        </div>
      )}

      {/* Back link */}
      <a
        href="/"
        className="absolute bottom-4 left-4 z-10 text-xs text-neon-blue border border-neon-blue/30 px-3 py-1.5 rounded-lg hover:bg-neon-blue/10 transition-colors"
      >
        ← Back to Tutor
      </a>
    </div>
  )
}
