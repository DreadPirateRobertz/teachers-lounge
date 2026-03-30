'use client'

import { useRef, useEffect, useCallback, useState } from 'react'
import { createBattleEngine, type BattleEngine } from '@/lib/battle-engine'

interface UseBattleEngineOptions {
  /** Start the loop immediately after mount (default: false — wait for assets) */
  autoStart?: boolean
}

/**
 * React hook that manages BattleEngine lifecycle.
 * Attaches to a canvas ref, handles resize, and cleans up on unmount.
 */
export function useBattleEngine(opts: UseBattleEngineOptions = {}) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const engineRef = useRef<BattleEngine | null>(null)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const engine = createBattleEngine({ canvas })
    engineRef.current = engine
    setReady(true)

    if (opts.autoStart) engine.start()

    const onResize = () => engine.scene.resize()
    window.addEventListener('resize', onResize)

    return () => {
      window.removeEventListener('resize', onResize)
      engine.dispose()
      engineRef.current = null
      setReady(false)
    }
  }, [opts.autoStart])

  const getEngine = useCallback((): BattleEngine | null => {
    return engineRef.current
  }, [])

  return { canvasRef, getEngine, ready }
}
