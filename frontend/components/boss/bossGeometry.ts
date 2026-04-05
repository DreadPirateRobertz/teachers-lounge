/**
 * Boss geometry factory — pure Three.js functions, no React.
 *
 * Each function builds a Three.js Group containing all meshes for a given
 * geometry type. Meshes use MeshPhongMaterial with boss color + emissive glow.
 */

import * as THREE from 'three'
import type { BossConfig, GeometryType } from './types'

/**
 * Create a MeshPhongMaterial with emissive glow using the boss's color scheme.
 * @param color - Primary hex color string.
 * @param emissive - Emissive hex color string for glow effect.
 */
function makeMaterial(color: string, emissive: string): THREE.MeshPhongMaterial {
  return new THREE.MeshPhongMaterial({
    color: new THREE.Color(color),
    emissive: new THREE.Color(emissive),
    emissiveIntensity: 0.4,
    shininess: 80,
  })
}

/**
 * Build the 'atom' geometry: an IcosahedronGeometry nucleus surrounded by
 * three TorusGeometry electron rings at distinct rotations.
 * @param config - Boss configuration providing color values.
 */
function buildAtom(config: BossConfig): THREE.Group {
  const group = new THREE.Group()
  const mat = makeMaterial(config.color, config.accentColor)

  // Nucleus
  const nucleus = new THREE.Mesh(new THREE.IcosahedronGeometry(0.5, 1), mat)
  group.add(nucleus)

  // Three electron rings at different axial rotations
  const ringMat = makeMaterial(config.accentColor, config.color)
  const ringAngles: [number, number, number][] = [
    [0, 0, 0],
    [Math.PI / 2, 0, 0],
    [Math.PI / 4, Math.PI / 4, 0],
  ]
  for (const [rx, ry, rz] of ringAngles) {
    const ring = new THREE.Mesh(new THREE.TorusGeometry(0.9, 0.04, 8, 48), ringMat)
    ring.rotation.x = rx
    ring.rotation.y = ry
    ring.rotation.z = rz
    group.add(ring)
  }

  return group
}

/**
 * Build the 'dumbbell' geometry: two SphereGeometry balls connected by a
 * CylinderGeometry shaft.
 * @param config - Boss configuration providing color values.
 */
function buildDumbbell(config: BossConfig): THREE.Group {
  const group = new THREE.Group()
  const mat = makeMaterial(config.color, config.accentColor)
  const shaftMat = makeMaterial(config.accentColor, config.color)

  // Left ball
  const left = new THREE.Mesh(new THREE.SphereGeometry(0.5, 16, 16), mat)
  left.position.set(-1.0, 0, 0)
  group.add(left)

  // Right ball
  const right = new THREE.Mesh(new THREE.SphereGeometry(0.5, 16, 16), mat)
  right.position.set(1.0, 0, 0)
  group.add(right)

  // Connecting shaft
  const shaft = new THREE.Mesh(new THREE.CylinderGeometry(0.1, 0.1, 1.8, 12), shaftMat)
  shaft.rotation.z = Math.PI / 2
  group.add(shaft)

  return group
}

/**
 * Build the 'grid' geometry: a 3×3 array of BoxGeometry cubes.
 * @param config - Boss configuration providing color values.
 */
function buildGrid(config: BossConfig): THREE.Group {
  const group = new THREE.Group()
  const mat = makeMaterial(config.color, config.accentColor)
  const spacing = 0.65

  for (let row = 0; row < 3; row++) {
    for (let col = 0; col < 3; col++) {
      const cube = new THREE.Mesh(new THREE.BoxGeometry(0.4, 0.4, 0.4), mat)
      cube.position.set(
        (col - 1) * spacing,
        (row - 1) * spacing,
        0,
      )
      group.add(cube)
    }
  }

  return group
}

/**
 * Build the 'tetrahedron' geometry: a TetrahedronGeometry body with small
 * SphereGeometry markers at each of its four vertices.
 * @param config - Boss configuration providing color values.
 */
function buildTetrahedron(config: BossConfig): THREE.Group {
  const group = new THREE.Group()
  const mat = makeMaterial(config.color, config.accentColor)
  const vertMat = makeMaterial(config.accentColor, config.color)

  // Central tetrahedron
  const body = new THREE.Mesh(new THREE.TetrahedronGeometry(0.8, 0), mat)
  group.add(body)

  // Vertex markers — positions derived from a unit tetrahedron scaled to r=0.8
  const r = 0.8
  const vertexPositions: [number, number, number][] = [
    [0, r, 0],
    [r * Math.sin((2 * Math.PI) / 3) * Math.sin(Math.acos(-1 / 3)), r * (-1 / 3), 0],
    [
      r * Math.sin((4 * Math.PI) / 3) * Math.sin(Math.acos(-1 / 3)),
      r * (-1 / 3),
      r * Math.cos((4 * Math.PI) / 3) * Math.sin(Math.acos(-1 / 3)),
    ],
    [0, r * (-1 / 3), -r * Math.sin(Math.acos(-1 / 3))],
  ]

  for (const pos of vertexPositions) {
    const sphere = new THREE.Mesh(new THREE.SphereGeometry(0.12, 8, 8), vertMat)
    sphere.position.set(...pos)
    group.add(sphere)
  }

  return group
}

/**
 * Build the 'flask' geometry: a CylinderGeometry neck/body with a
 * SphereGeometry bulb at the base.
 * @param config - Boss configuration providing color values.
 */
function buildFlask(config: BossConfig): THREE.Group {
  const group = new THREE.Group()
  const mat = makeMaterial(config.color, config.accentColor)
  const bulbMat = makeMaterial(config.accentColor, config.color)

  // Flask body (tapered cylinder)
  const body = new THREE.Mesh(new THREE.CylinderGeometry(0.15, 0.45, 1.2, 16), mat)
  body.position.set(0, 0.3, 0)
  group.add(body)

  // Bulb at base
  const bulb = new THREE.Mesh(new THREE.SphereGeometry(0.55, 16, 16), bulbMat)
  bulb.position.set(0, -0.45, 0)
  group.add(bulb)

  return group
}

/** Dispatch map from geometry type to builder function. */
const BUILDERS: Record<GeometryType, (config: BossConfig) => THREE.Group> = {
  atom: buildAtom,
  dumbbell: buildDumbbell,
  grid: buildGrid,
  tetrahedron: buildTetrahedron,
  flask: buildFlask,
}

/**
 * Create the Three.js Group for the given boss config.
 * The returned group is ready to be added to a Three.js Scene.
 * @param config - Full boss config including geometryType and color values.
 */
export function createBossGeometry(config: BossConfig): THREE.Group {
  const group = BUILDERS[config.geometryType](config)
  group.scale.setScalar(config.scale)
  return group
}
