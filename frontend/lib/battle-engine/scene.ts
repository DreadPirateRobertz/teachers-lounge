import * as THREE from 'three'
import type { SceneConfig } from './types'

const DEFAULTS = {
  antialias: true,
  backgroundColor: 0x0a0a1a,
  fov: 60,
  maxPixelRatio: 2,
} as const

/**
 * Creates and manages the Three.js scene, camera, renderer, and lighting
 * for a boss battle. Call `dispose()` on unmount to release GPU resources.
 */
export function createBattleScene(config: SceneConfig) {
  const opts = { ...DEFAULTS, ...config }

  const renderer = new THREE.WebGLRenderer({
    canvas: opts.canvas,
    antialias: opts.antialias,
    alpha: false,
  })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, opts.maxPixelRatio))
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 1.2

  const scene = new THREE.Scene()
  scene.background = new THREE.Color(opts.backgroundColor)
  scene.fog = new THREE.FogExp2(opts.backgroundColor, 0.015)

  const { width, height } = opts.canvas.getBoundingClientRect()
  const camera = new THREE.PerspectiveCamera(
    opts.fov,
    width / height || 1,
    0.1,
    200,
  )
  camera.position.set(0, 3, 8)
  camera.lookAt(0, 1, 0)

  // Ambient fill — dim neon blue tint
  const ambient = new THREE.AmbientLight(0x00aaff, 0.3)
  scene.add(ambient)

  // Key light — bright directional from above-right
  const key = new THREE.DirectionalLight(0xffffff, 1.0)
  key.position.set(4, 8, 4)
  scene.add(key)

  // Rim light — neon pink from behind for sci-fi edge
  const rim = new THREE.PointLight(0xff00aa, 0.6, 20)
  rim.position.set(-3, 4, -4)
  scene.add(rim)

  renderer.setSize(width, height)

  function resize() {
    const { width: w, height: h } = opts.canvas.getBoundingClientRect()
    if (w === 0 || h === 0) return
    renderer.setSize(w, h)
    camera.aspect = w / h
    camera.updateProjectionMatrix()
  }

  function render() {
    renderer.render(scene, camera)
  }

  function dispose() {
    renderer.dispose()
    scene.traverse((obj) => {
      if (obj instanceof THREE.Mesh) {
        obj.geometry.dispose()
        const mat = obj.material
        if (Array.isArray(mat)) mat.forEach((m) => m.dispose())
        else mat.dispose()
      }
    })
  }

  return { renderer, scene, camera, resize, render, dispose }
}

export type BattleScene = ReturnType<typeof createBattleScene>
