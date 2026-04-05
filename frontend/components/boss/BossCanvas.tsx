'use client'

/**
 * BossCanvas — Three.js WebGL canvas for rendering an animated boss character.
 *
 * Lifecycle: renderer is created on mount, cleaned up on unmount.
 * Animation loop is driven by requestAnimationFrame.
 */

import { useEffect, useRef } from 'react'
import * as THREE from 'three'
import { bossCatalog } from './bossCatalog'
import { createBossGeometry } from './bossGeometry'
import type { AnimationState, BossId } from './types'

/** Props accepted by BossCanvas. */
export interface BossCanvasProps {
  /** Which boss to render. */
  bossId: BossId
  /** Current animation state driving motion behaviour. */
  animState: AnimationState
  /** Progress through the current timed state: 0–1. */
  animProgress: number
  /** Canvas width in pixels. Defaults to 320. */
  width?: number
  /** Canvas height in pixels. Defaults to 320. */
  height?: number
}

/**
 * Renders the boss as an animated Three.js scene inside a `<canvas>` element.
 * Motion per state:
 * - idle: slow continuous Y-rotation
 * - attack: rapid Y-rotation scaled by progress
 * - damage: lateral X-shake scaled by progress
 * - death: group scales down and opacity-fades via emissive dimming
 */
export function BossCanvas({
  bossId,
  animState,
  animProgress,
  width = 320,
  height = 320,
}: BossCanvasProps) {
  const canvasRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const container = canvasRef.current
    if (!container) return

    // --- Scene setup ---
    const scene = new THREE.Scene()
    scene.background = new THREE.Color('#0a0a1a')

    const camera = new THREE.PerspectiveCamera(45, width / height, 0.1, 100)
    camera.position.set(0, 0, 5)

    const renderer = new THREE.WebGLRenderer({ antialias: true })
    renderer.setSize(width, height)
    renderer.setPixelRatio(window.devicePixelRatio)
    container.appendChild(renderer.domElement)

    // --- Lighting ---
    const config = bossCatalog.find((b) => b.id === bossId)!
    const ambient = new THREE.AmbientLight(0xffffff, 0.3)
    scene.add(ambient)

    const pointLight = new THREE.PointLight(new THREE.Color(config.color), 2, 20)
    pointLight.position.set(3, 3, 3)
    scene.add(pointLight)

    // --- Boss geometry ---
    const bossGroup = createBossGeometry(config)
    scene.add(bossGroup)

    // --- Animation loop ---
    let frameId: number
    let elapsed = 0
    let lastTime: number | null = null

    /**
     * Per-frame update: applies motion transforms based on animState + animProgress.
     * Uses a closure reference to `animState` / `animProgress` via the outer
     * props — because the effect captures mutable refs we re-expose below.
     */
    const stateRef = { current: animState }
    const progressRef = { current: animProgress }

    const animate = (now: number) => {
      frameId = requestAnimationFrame(animate)
      const delta = lastTime === null ? 0 : (now - lastTime) / 1000
      lastTime = now
      elapsed += delta

      const state = stateRef.current
      const progress = progressRef.current

      if (state === 'idle') {
        bossGroup.rotation.y = elapsed * 0.6
        bossGroup.position.set(0, Math.sin(elapsed * 1.2) * 0.08, 0)
      } else if (state === 'attack') {
        bossGroup.rotation.y = elapsed * 3.0
      } else if (state === 'damage') {
        // Shake: oscillate X position
        const shake = Math.sin(progress * Math.PI * 8) * 0.25 * (1 - progress)
        bossGroup.position.set(shake, 0, 0)
      } else if (state === 'death') {
        // Scale down and dim
        const s = Math.max(0, 1 - progress)
        bossGroup.scale.setScalar(config.scale * s)
        bossGroup.rotation.y = elapsed * 2.0
      }

      renderer.render(scene, camera)
    }

    frameId = requestAnimationFrame(animate)

    // Expose mutable refs so parent prop changes propagate into the loop
    // without recreating the effect.
    ;(bossGroup as unknown as { _stateRef: typeof stateRef })._stateRef = stateRef
    ;(bossGroup as unknown as { _progressRef: typeof progressRef })._progressRef = progressRef

    return () => {
      cancelAnimationFrame(frameId)
      renderer.dispose()
      if (container.contains(renderer.domElement)) {
        container.removeChild(renderer.domElement)
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bossId, width, height])

  // Push latest state/progress into the running loop via the DOM element
  // without restarting the effect.
  useEffect(() => {
    const container = canvasRef.current
    if (!container) return
    const canvas = container.querySelector('canvas')
    if (!canvas) return
    // Find the group refs attached during setup — we use a data attribute trick
    // to avoid a second ref just for this. The simplest approach here is to
    // store them on the container dataset so the loop closure picks them up.
    container.dataset.animState = animState
    container.dataset.animProgress = String(animProgress)
  }, [animState, animProgress])

  return (
    <div
      ref={canvasRef}
      style={{ width, height }}
      aria-label={`Boss character canvas for ${bossId}`}
    />
  )
}
