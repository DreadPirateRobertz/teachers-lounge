'use client'

/**
 * BossScene.tsx
 *
 * Three.js WebGL canvas that renders a procedurally-generated boss entity.
 * Each boss uses a distinct geometry builder driven by BossCharacterLibrary.
 *
 * Animation state prop drives which loop is active:
 *   idle    — continuous ambient loop
 *   attack  — single fire, then returns to idle
 *   damage  — brief recoil flash, then returns to idle
 *   death   — plays once to completion (calls onDeathComplete)
 */

import { useEffect, useRef } from 'react'
import * as THREE from 'three'
import { type AnimationState, type BossVisualDef } from './BossCharacterLibrary'

/** Props for the BossScene canvas component. */
interface BossSceneProps {
  /** Full boss definition from BossCharacterLibrary. */
  boss: BossVisualDef
  /** Current animation state driven by battle events. */
  animationState: AnimationState
  /** Called when the death animation completes. */
  onDeathComplete?: () => void
  /** Canvas width in pixels. */
  width?: number
  /** Canvas height in pixels. */
  height?: number
}

// ─── Geometry Builders ───────────────────────────────────────────────────────

/**
 * Builds the THE ATOM scene: nucleus sphere + orbital rings + electron spheres.
 * Returns the root group and an update function called each frame.
 */
function buildAtom(def: BossVisualDef): {
  group: THREE.Group
  update: (t: number, state: AnimationState) => void
} {
  const group = new THREE.Group()
  const primary = new THREE.Color(def.primaryColor)
  const secondary = new THREE.Color(def.secondaryColor)

  // Nucleus
  const nucleusMat = new THREE.MeshPhongMaterial({
    color: primary,
    emissive: primary,
    emissiveIntensity: 0.6,
    shininess: 80,
  })
  const nucleus = new THREE.Mesh(new THREE.SphereGeometry(0.5, 32, 32), nucleusMat)
  group.add(nucleus)

  // Three orbital rings at different tilts
  const ringAngles = [0, Math.PI / 3, (2 * Math.PI) / 3]
  const rings: THREE.Group[] = []
  const electrons: THREE.Mesh[] = []

  ringAngles.forEach((angle) => {
    const ring = new THREE.Group()
    ring.rotation.x = angle

    const torusMat = new THREE.MeshPhongMaterial({
      color: secondary,
      emissive: secondary,
      emissiveIntensity: 0.3,
      transparent: true,
      opacity: 0.5,
    })
    const torus = new THREE.Mesh(new THREE.TorusGeometry(1.2, 0.03, 8, 48), torusMat)
    ring.add(torus)

    // Electron on this ring
    const eMat = new THREE.MeshPhongMaterial({
      color: secondary,
      emissive: secondary,
      emissiveIntensity: 0.8,
    })
    const electron = new THREE.Mesh(new THREE.SphereGeometry(0.1, 8, 8), eMat)
    electron.position.set(1.2, 0, 0)
    ring.add(electron)
    electrons.push(electron)
    rings.push(ring)
    group.add(ring)
  })

  const speeds = [0.8, 1.2, 0.5]

  function update(t: number, state: AnimationState) {
    const params = def.animations[state]
    const speed = params.rotationSpeed

    rings.forEach((ring, i) => {
      ring.rotation.y += 0.016 * speed * speeds[i]
      electrons[i].position.x = Math.cos(t * speed * speeds[i]) * 1.2
      electrons[i].position.z = Math.sin(t * speed * speeds[i]) * 1.2
    })

    // Nucleus pulse in idle/damage
    const pulse = 1 + Math.sin(t * 3) * params.oscillationAmp
    nucleus.scale.setScalar(pulse)

    if (state === 'damage') {
      const flash = Math.sin(t * 20) > 0
      nucleusMat.emissiveIntensity = flash ? 2.0 : 0.6
    } else {
      nucleusMat.emissiveIntensity = 0.6
    }
  }

  return { group, update }
}

/**
 * Builds THE BONDER: two spheres connected by bond cylinders.
 */
function buildBonder(def: BossVisualDef): {
  group: THREE.Group
  update: (t: number, state: AnimationState) => void
} {
  const group = new THREE.Group()
  const primary = new THREE.Color(def.primaryColor)
  const secondary = new THREE.Color(def.secondaryColor)

  const headMat = new THREE.MeshPhongMaterial({
    color: primary,
    emissive: primary,
    emissiveIntensity: 0.5,
    shininess: 60,
  })

  const head1 = new THREE.Mesh(new THREE.SphereGeometry(0.55, 24, 24), headMat.clone())
  const head2 = new THREE.Mesh(new THREE.SphereGeometry(0.55, 24, 24), headMat.clone())
  head1.position.set(-1.0, 0, 0)
  head2.position.set(1.0, 0, 0)

  const bondMat = new THREE.MeshPhongMaterial({
    color: secondary,
    emissive: secondary,
    emissiveIntensity: 0.4,
  })
  const bond = new THREE.Mesh(new THREE.CylinderGeometry(0.08, 0.08, 2.0, 12), bondMat)
  bond.rotation.z = Math.PI / 2

  group.add(head1, head2, bond)

  function update(t: number, state: AnimationState) {
    const params = def.animations[state]
    const spread = state === 'attack' ? Math.abs(Math.sin(t * 4)) * 1.5 : 0
    const base = 1.0 + spread
    head1.position.x = -base
    head2.position.x = base
    bond.scale.x = base / 1.0

    group.rotation.y += 0.01 * params.rotationSpeed

    const osc = Math.sin(t * 2) * params.oscillationAmp
    head1.position.y = osc
    head2.position.y = -osc

    if (state === 'damage') {
      const flash = Math.sin(t * 25) > 0
      ;(head1.material as THREE.MeshPhongMaterial).emissiveIntensity = flash ? 2.0 : 0.5
      ;(head2.material as THREE.MeshPhongMaterial).emissiveIntensity = flash ? 2.0 : 0.5
    }
  }

  return { group, update }
}

/**
 * Builds NAME LORD: morphing icosahedron with vertex displacement.
 */
function buildNameLord(def: BossVisualDef): {
  group: THREE.Group
  update: (t: number, state: AnimationState) => void
} {
  const group = new THREE.Group()
  const primary = new THREE.Color(def.primaryColor)

  const geo = new THREE.IcosahedronGeometry(0.9, 2)
  const mat = new THREE.MeshPhongMaterial({
    color: primary,
    emissive: primary,
    emissiveIntensity: 0.4,
    wireframe: false,
    shininess: 100,
  })
  const mesh = new THREE.Mesh(geo, mat)
  group.add(mesh)

  // Wireframe overlay for the "shape-shifting" feel
  const wireMat = new THREE.MeshBasicMaterial({
    color: def.secondaryColor,
    wireframe: true,
    transparent: true,
    opacity: 0.3,
  })
  const wire = new THREE.Mesh(new THREE.IcosahedronGeometry(0.92, 2), wireMat)
  group.add(wire)

  const posAttr = geo.attributes.position as THREE.BufferAttribute
  const origPositions = Float32Array.from(posAttr.array)

  function update(t: number, state: AnimationState) {
    const params = def.animations[state]
    const amp = params.oscillationAmp * 0.15

    // Morph vertices
    for (let i = 0; i < posAttr.count; i++) {
      const ix = origPositions[i * 3]
      const iy = origPositions[i * 3 + 1]
      const iz = origPositions[i * 3 + 2]
      const noise = Math.sin(t * params.rotationSpeed + ix * 3 + iy * 3 + iz * 3) * amp
      posAttr.setXYZ(i, ix + ix * noise, iy + iy * noise, iz + iz * noise)
    }
    posAttr.needsUpdate = true
    geo.computeVertexNormals()

    mesh.rotation.y += 0.012 * params.rotationSpeed
    mesh.rotation.x += 0.005 * params.rotationSpeed
    wire.rotation.y = mesh.rotation.y
    wire.rotation.x = mesh.rotation.x

    if (state === 'damage') {
      const flash = Math.sin(t * 22) > 0
      mat.emissiveIntensity = flash ? 2.5 : 0.4
    } else {
      mat.emissiveIntensity = 0.4
    }
  }

  return { group, update }
}

/**
 * Builds THE STEREOCHEMIST: two mirrored tetrahedra on a shared axis.
 */
function buildStereochemist(def: BossVisualDef): {
  group: THREE.Group
  update: (t: number, state: AnimationState) => void
} {
  const group = new THREE.Group()
  const primary = new THREE.Color(def.primaryColor)
  const secondary = new THREE.Color(def.secondaryColor)

  const mat1 = new THREE.MeshPhongMaterial({
    color: primary,
    emissive: primary,
    emissiveIntensity: 0.5,
    transparent: true,
    opacity: 0.9,
  })
  const mat2 = new THREE.MeshPhongMaterial({
    color: secondary,
    emissive: secondary,
    emissiveIntensity: 0.5,
    transparent: true,
    opacity: 0.9,
  })

  const geo = new THREE.TetrahedronGeometry(0.7, 0)
  const tet1 = new THREE.Mesh(geo, mat1)
  const tet2 = new THREE.Mesh(geo.clone(), mat2)
  tet1.position.set(-0.9, 0, 0)
  tet2.position.set(0.9, 0, 0)
  tet2.scale.x = -1 // Mirror chirality

  // Shared axis connector
  const axisMat = new THREE.MeshBasicMaterial({
    color: def.accentColor,
    transparent: true,
    opacity: 0.4,
  })
  const axis = new THREE.Mesh(new THREE.CylinderGeometry(0.02, 0.02, 1.8, 8), axisMat)
  axis.rotation.z = Math.PI / 2

  group.add(tet1, tet2, axis)

  function update(t: number, state: AnimationState) {
    const params = def.animations[state]
    group.rotation.y += 0.01 * params.rotationSpeed

    tet1.rotation.x = t * params.rotationSpeed * 0.7
    tet2.rotation.x = -(t * params.rotationSpeed * 0.7)

    if (state === 'attack') {
      const advance = Math.abs(Math.sin(t * 3)) * 0.5
      tet2.position.x = 0.9 + advance
    } else {
      tet2.position.x = 0.9
    }

    if (state === 'damage') {
      const flash = Math.sin(t * 20) > 0
      mat1.emissiveIntensity = flash ? 2.0 : 0.5
      mat2.emissiveIntensity = flash ? 2.0 : 0.5
    }
  }

  return { group, update }
}

/**
 * Builds THE REACTOR: torus vessel with internal particle system.
 */
function buildReactor(def: BossVisualDef): {
  group: THREE.Group
  update: (t: number, state: AnimationState) => void
} {
  const group = new THREE.Group()
  const primary = new THREE.Color(def.primaryColor)
  const secondary = new THREE.Color(def.secondaryColor)

  const vesselMat = new THREE.MeshPhongMaterial({
    color: primary,
    emissive: primary,
    emissiveIntensity: 0.3,
    transparent: true,
    opacity: 0.75,
    shininess: 120,
  })
  const vessel = new THREE.Mesh(new THREE.TorusGeometry(1.0, 0.35, 16, 48), vesselMat)
  group.add(vessel)

  // Inner glow sphere
  const innerMat = new THREE.MeshPhongMaterial({
    color: secondary,
    emissive: secondary,
    emissiveIntensity: 0.8,
    transparent: true,
    opacity: 0.5,
  })
  const inner = new THREE.Mesh(new THREE.SphereGeometry(0.4, 16, 16), innerMat)
  group.add(inner)

  // Particle ring inside the torus
  const particleCount = 20
  const particles: THREE.Mesh[] = []
  const pMat = new THREE.MeshBasicMaterial({ color: secondary })
  for (let i = 0; i < particleCount; i++) {
    const p = new THREE.Mesh(new THREE.SphereGeometry(0.05, 4, 4), pMat)
    group.add(p)
    particles.push(p)
  }

  function update(t: number, state: AnimationState) {
    const params = def.animations[state]
    vessel.rotation.x += 0.008 * params.rotationSpeed
    vessel.rotation.z += 0.012 * params.rotationSpeed

    particles.forEach((p, i) => {
      const angle = (i / particleCount) * Math.PI * 2 + t * params.rotationSpeed
      const burst = state === 'attack' ? 1 + Math.abs(Math.sin(t * 5)) * 1.5 : 1.0
      p.position.set(
        Math.cos(angle) * 1.0 * burst,
        Math.sin(angle) * 0.3,
        Math.sin(angle) * 1.0 * burst,
      )
    })

    const pulse = 1 + Math.sin(t * 4) * params.oscillationAmp
    inner.scale.setScalar(pulse)

    if (state === 'damage') {
      const flash = Math.sin(t * 18) > 0
      vesselMat.emissiveIntensity = flash ? 2.5 : 0.3
    } else {
      vesselMat.emissiveIntensity = 0.3
    }
  }

  return { group, update }
}

/**
 * Builds the FINAL BOSS: composite entity combining elements of all chapter bosses.
 */
function buildFinalBoss(def: BossVisualDef): {
  group: THREE.Group
  update: (t: number, state: AnimationState) => void
} {
  const group = new THREE.Group()
  const primary = new THREE.Color(def.primaryColor)
  const secondary = new THREE.Color(def.secondaryColor)
  const accent = new THREE.Color(def.accentColor)

  // Core: large icosahedron (Name Lord inheritance)
  const coreMat = new THREE.MeshPhongMaterial({
    color: primary,
    emissive: primary,
    emissiveIntensity: 0.6,
    shininess: 60,
  })
  const core = new THREE.Mesh(new THREE.IcosahedronGeometry(0.65, 1), coreMat)
  group.add(core)

  // Orbital ring (Atom inheritance)
  const ringMat = new THREE.MeshPhongMaterial({
    color: secondary,
    emissive: secondary,
    emissiveIntensity: 0.5,
    transparent: true,
    opacity: 0.6,
  })
  const ring = new THREE.Mesh(new THREE.TorusGeometry(1.3, 0.06, 8, 48), ringMat)
  ring.rotation.x = Math.PI / 3
  group.add(ring)

  // Mirrored tetrahedra arms (Stereochemist inheritance)
  const armMat1 = new THREE.MeshPhongMaterial({
    color: secondary,
    emissive: secondary,
    emissiveIntensity: 0.4,
  })
  const armMat2 = new THREE.MeshPhongMaterial({
    color: accent,
    emissive: accent,
    emissiveIntensity: 0.4,
  })
  const arm1 = new THREE.Mesh(new THREE.TetrahedronGeometry(0.4, 0), armMat1)
  const arm2 = new THREE.Mesh(new THREE.TetrahedronGeometry(0.4, 0), armMat2)
  arm1.position.set(1.1, 0.3, 0)
  arm2.position.set(-1.1, -0.3, 0)
  arm2.scale.x = -1
  group.add(arm1, arm2)

  // Reactive aura (Reactor inheritance)
  const auraMat = new THREE.MeshPhongMaterial({
    color: accent,
    emissive: accent,
    emissiveIntensity: 0.2,
    transparent: true,
    opacity: 0.2,
    wireframe: true,
  })
  const aura = new THREE.Mesh(new THREE.SphereGeometry(1.6, 16, 16), auraMat)
  group.add(aura)

  function update(t: number, state: AnimationState) {
    const params = def.animations[state]
    core.rotation.y += 0.015 * params.rotationSpeed
    core.rotation.x += 0.008 * params.rotationSpeed
    ring.rotation.z += 0.02 * params.rotationSpeed
    arm1.rotation.x = t * params.rotationSpeed * 0.6
    arm2.rotation.x = -(t * params.rotationSpeed * 0.6)
    aura.rotation.y -= 0.005 * params.rotationSpeed

    const pulse = 1 + Math.sin(t * 2) * params.oscillationAmp
    core.scale.setScalar(pulse)
    aura.scale.setScalar(1 + Math.sin(t * 1.5) * 0.05)

    if (state === 'damage') {
      const flash = Math.sin(t * 20) > 0
      coreMat.emissiveIntensity = flash ? 3.0 : 0.6
      auraMat.opacity = flash ? 0.5 : 0.2
    } else {
      coreMat.emissiveIntensity = 0.6
      auraMat.opacity = 0.2
    }
  }

  return { group, update }
}

// ─── Geometry Dispatch ────────────────────────────────────────────────────────

function buildBossGeometry(def: BossVisualDef): {
  group: THREE.Group
  update: (t: number, state: AnimationState) => void
} {
  switch (def.geometry) {
    case 'atom':
      return buildAtom(def)
    case 'bonder':
      return buildBonder(def)
    case 'name_lord':
      return buildNameLord(def)
    case 'stereochemist':
      return buildStereochemist(def)
    case 'reactor':
      return buildReactor(def)
    case 'final_boss':
      return buildFinalBoss(def)
    default:
      return buildAtom(def)
  }
}

// ─── Scene Component ──────────────────────────────────────────────────────────

/**
 * BossScene renders a Three.js WebGL canvas for the given boss.
 * The parent drives animation state; this component only renders.
 */
export default function BossScene({
  boss,
  animationState,
  onDeathComplete,
  width = 400,
  height = 400,
}: BossSceneProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const stateRef = useRef(animationState)

  // Keep state ref current without rebuilding the scene.
  stateRef.current = animationState

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    // Scene setup
    const scene = new THREE.Scene()
    scene.background = new THREE.Color('#0a0a1a')

    const camera = new THREE.PerspectiveCamera(60, width / height, 0.1, 100)
    camera.position.set(0, 0.5, 4.5)

    const renderer = new THREE.WebGLRenderer({ canvas, antialias: true })
    renderer.setSize(width, height)
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))

    // Lighting
    const ambient = new THREE.AmbientLight(0x111133, 1.5)
    scene.add(ambient)

    const point1 = new THREE.PointLight(boss.primaryColor, 3, 12)
    point1.position.set(3, 3, 3)
    scene.add(point1)

    const point2 = new THREE.PointLight(boss.secondaryColor, 2, 10)
    point2.position.set(-3, -2, 2)
    scene.add(point2)

    // Boss mesh
    const { group, update } = buildBossGeometry(boss)
    group.scale.setScalar(boss.scale)
    scene.add(group)

    // Animation loop
    const startTime = performance.now() / 1000
    let deathStarted = false
    let deathTimer = 0
    let animId = 0

    function animate() {
      animId = requestAnimationFrame(animate)
      const elapsed = performance.now() / 1000 - startTime
      const state = stateRef.current

      // Death sequence: play once then notify parent
      if (state === 'death') {
        if (!deathStarted) {
          deathStarted = true
          deathTimer = elapsed
        }
        const deathParams = boss.animations.death
        if (elapsed - deathTimer >= deathParams.cycleSec) {
          onDeathComplete?.()
          cancelAnimationFrame(animId)
          return
        }
        // Scale down + spin out + tilt forward as the boss disintegrates.
        // Eased so the effect is gentle at the start and accelerates into
        // the collapse — feels like collapse rather than a linear shrink.
        const progress = (elapsed - deathTimer) / deathParams.cycleSec
        const eased = progress * progress
        group.scale.setScalar(boss.scale * (1 - eased * 0.95))
        group.rotation.y += 0.15 * (1 + eased * 4)
        group.rotation.x = eased * Math.PI * 0.4
      }

      update(elapsed, state)
      renderer.render(scene, camera)
    }

    animate()

    return () => {
      cancelAnimationFrame(animId)
      renderer.dispose()
    }
    // Re-mount if boss changes (different boss fight).
    // animationState changes are handled via stateRef.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [boss.id, width, height])

  return (
    <canvas
      ref={canvasRef}
      width={width}
      height={height}
      className="rounded-lg"
      aria-label={`${boss.name} boss battle scene`}
    />
  )
}
