'use client'

/**
 * MoleculeViewerCanvas — Three.js / React-Three-Fiber canvas for 3-D molecule rendering.
 *
 * This file is dynamically imported by MoleculeViewer (ssr: false) so that
 * Three.js never runs on the server, where WebGL is unavailable.
 *
 * @module MoleculeViewerCanvas
 */

import React, { useState, useRef } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import { OrbitControls, Sphere, Cylinder, Html } from '@react-three/drei'
import * as THREE from 'three'
import {
  type MoleculeData,
  type Atom,
  type Bond,
  elementColor,
  elementRadius,
} from './moleculeGeometry'

// ── AtomSphere ────────────────────────────────────────────────────────────────

interface AtomSphereProps {
  /** Atom data including element and position */
  atom: Atom
  /** Index of this atom in the molecule's atom array */
  index: number
}

/**
 * Renders a single atom as a CPK-colored sphere.
 * Click to toggle an element info tooltip.
 *
 * @param props - AtomSphereProps
 */
function AtomSphere({ atom, index }: AtomSphereProps) {
  const [hovered, setHovered] = useState(false)
  const [clicked, setClicked] = useState(false)
  const meshRef = useRef<THREE.Mesh>(null)

  const color = elementColor(atom.element)
  const radius = elementRadius(atom.element)

  useFrame(() => {
    if (meshRef.current) {
      const mat = meshRef.current.material as THREE.MeshStandardMaterial
      mat.emissiveIntensity = hovered ? 0.4 : 0.1
    }
  })

  return (
    <Sphere
      ref={meshRef}
      args={[radius, 32, 32]}
      position={atom.position}
      onClick={() => setClicked((c) => !c)}
      onPointerOver={() => setHovered(true)}
      onPointerOut={() => setHovered(false)}
    >
      <meshStandardMaterial
        color={color}
        roughness={0.35}
        metalness={0.1}
        emissive={color}
        emissiveIntensity={0.1}
      />
      {clicked && (
        <Html distanceFactor={4}>
          <div
            style={{
              background: '#0a0a1a',
              border: '1px solid #252560',
              color: '#c8c8e8',
              borderRadius: 4,
              padding: '4px 8px',
              fontSize: 11,
              fontFamily: 'monospace',
              whiteSpace: 'nowrap',
              pointerEvents: 'none',
            }}
          >
            <span style={{ color: '#00aaff', fontWeight: 'bold' }}>{atom.element}</span>
            {' — atom '}
            {index}
            <br />
            [{atom.position.map((v) => v.toFixed(2)).join(', ')}] Å
          </div>
        </Html>
      )}
    </Sphere>
  )
}

// ── BondCylinder ──────────────────────────────────────────────────────────────

interface BondCylinderProps {
  /** Bond data */
  bond: Bond
  /** Atoms array to look up positions from */
  atoms: Atom[]
}

/**
 * Renders a bond between two atoms as one or more thin cylinders.
 * Double/triple bonds are rendered as parallel cylinders offset slightly.
 *
 * @param props - BondCylinderProps
 */
function BondCylinder({ bond, atoms }: BondCylinderProps) {
  const a = atoms[bond.from]
  const b = atoms[bond.to]

  const start = new THREE.Vector3(...a.position)
  const end = new THREE.Vector3(...b.position)
  const mid = start.clone().add(end).multiplyScalar(0.5)
  const dir = end.clone().sub(start)
  const length = dir.length()

  // Quaternion to rotate the default Y-axis cylinder to point from a → b
  const up = new THREE.Vector3(0, 1, 0)
  const quaternion = new THREE.Quaternion().setFromUnitVectors(up, dir.normalize())

  const offsets: [number, number][] =
    bond.order === 1
      ? [[0, 0]]
      : bond.order === 2
        ? [
            [-0.06, 0],
            [0.06, 0],
          ]
        : [
            [-0.1, 0],
            [0, 0],
            [0.1, 0],
          ]

  return (
    <>
      {offsets.map(([ox, oz], i) => (
        <Cylinder
          key={i}
          args={[0.04, 0.04, length, 8]}
          position={[mid.x + ox, mid.y, mid.z + oz]}
          quaternion={quaternion}
        >
          <meshStandardMaterial color="#555577" roughness={0.8} metalness={0.0} />
        </Cylinder>
      ))}
    </>
  )
}

// ── MoleculeScene ─────────────────────────────────────────────────────────────

interface MoleculeSceneProps {
  /** Molecule data to render */
  data: MoleculeData
}

/**
 * Renders all atoms and bonds for a molecule inside a Three.js scene.
 *
 * @param props - MoleculeSceneProps
 */
function MoleculeScene({ data }: MoleculeSceneProps) {
  return (
    <>
      <ambientLight intensity={0.6} />
      <pointLight position={[5, 5, 5]} intensity={1.2} />
      <pointLight position={[-5, -3, -5]} intensity={0.4} color="#00aaff" />
      <OrbitControls enablePan={false} enableZoom={true} enableRotate={true} />
      {data.atoms.map((atom, i) => (
        <AtomSphere key={i} atom={atom} index={i} />
      ))}
      {data.bonds.map((bond, i) => (
        <BondCylinder key={i} bond={bond} atoms={data.atoms} />
      ))}
    </>
  )
}

// ── MoleculeViewerCanvas ──────────────────────────────────────────────────────

interface MoleculeViewerCanvasProps {
  /** Resolved molecule data to render */
  data: MoleculeData
  /** Canvas width in pixels */
  width: number
  /** Canvas height in pixels */
  height: number
}

/**
 * Inner canvas component rendered client-side only via dynamic import.
 *
 * Creates a React-Three-Fiber Canvas with dark Neon Arcade background,
 * ambient glow lighting, and OrbitControls for drag-to-rotate.
 *
 * @param props - MoleculeViewerCanvasProps
 */
export default function MoleculeViewerCanvas({ data, width, height }: MoleculeViewerCanvasProps) {
  return (
    <Canvas
      style={{ width, height, background: '#0a0a0f', borderRadius: 8 }}
      camera={{ position: [0, 0, 6], fov: 50 }}
      gl={{ antialias: true, alpha: false }}
    >
      <MoleculeScene data={data} />
    </Canvas>
  )
}
