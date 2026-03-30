'use client'

import { useBattleEngine } from './useBattleEngine'
import type { BattleEngine } from '@/lib/battle-engine'
import { useEffect } from 'react'

interface BattleCanvasProps {
  /** Called once the engine is initialized and ready */
  onReady?: (engine: BattleEngine) => void
  className?: string
}

/**
 * Full-viewport Three.js canvas for boss battles.
 * Manages the engine lifecycle and exposes it via onReady callback.
 */
export function BattleCanvas({ onReady, className }: BattleCanvasProps) {
  const { canvasRef, getEngine, ready } = useBattleEngine()

  useEffect(() => {
    if (!ready) return
    const engine = getEngine()
    if (engine) onReady?.(engine)
  }, [ready, getEngine, onReady])

  return (
    <canvas
      ref={canvasRef}
      className={className ?? 'absolute inset-0 w-full h-full'}
    />
  )
}
