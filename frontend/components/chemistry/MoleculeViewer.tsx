'use client'

/**
 * MoleculeViewer — interactive 3-D molecule viewer for the Neon Arcade chat.
 *
 * Wraps the Three.js canvas in a Next.js dynamic import (ssr: false) so the
 * WebGL context is never created on the server. Atoms are rendered as CPK
 * spheres; bonds as cylinders. Click any atom to see its element and position.
 *
 * Supported molecule queries: name, formula, or common alias (case-insensitive).
 * Example:  <MoleculeViewer molecule="water" />
 *           <MoleculeViewer molecule="H2O" />
 *           <MoleculeViewer molecule="benzene" width={500} height={400} />
 *
 * @module MoleculeViewer
 */

import React from 'react'
import dynamic from 'next/dynamic'
import { findMolecule } from './moleculeGeometry'

/**
 * Dynamically imported canvas — SSR disabled so Three.js never runs server-side.
 * Renders the actual R3F Canvas + scene.
 */
const MoleculeViewerCanvas = dynamic(() => import('./MoleculeViewerCanvas'), { ssr: false })

// ── Props ─────────────────────────────────────────────────────────────────────

/** Props for the MoleculeViewer component. */
export interface MoleculeViewerProps {
  /** Molecule key from MOLECULES record, or formula, or name (case-insensitive). */
  molecule: string
  /** Canvas width in pixels (default 400). */
  width?: number
  /** Canvas height in pixels (default 300). */
  height?: number
}

// ── Component ─────────────────────────────────────────────────────────────────

/**
 * MoleculeViewer component — renders an interactive 3-D CPK model.
 *
 * Looks up the molecule via `findMolecule(molecule)`. If not found, shows an
 * "UNKNOWN MOLECULE" fallback in the Neon Arcade style.
 *
 * @param props - MoleculeViewerProps
 * @returns A styled container with the 3-D canvas, or a fallback message.
 */
export default function MoleculeViewer({
  molecule,
  width = 400,
  height = 300,
}: MoleculeViewerProps) {
  const data = findMolecule(molecule)

  return (
    <div
      style={{ width, height }}
      className="relative rounded-lg overflow-hidden border border-border-dim bg-bg-deep"
    >
      {data ? (
        <>
          {/* molecule label */}
          <div className="absolute top-2 left-3 z-10 text-[10px] font-mono text-neon-blue font-bold pointer-events-none">
            {data.formula} — {data.name.toUpperCase()}
          </div>
          <MoleculeViewerCanvas data={data} width={width} height={height} />
          {/* hint */}
          <div className="absolute bottom-2 right-3 z-10 text-[9px] font-mono text-text-dim pointer-events-none">
            drag to rotate · click atom for info
          </div>
        </>
      ) : (
        <div className="flex items-center justify-center w-full h-full">
          <span className="font-mono text-sm text-neon-pink font-bold tracking-widest">
            UNKNOWN MOLECULE
          </span>
        </div>
      )}
    </div>
  )
}
