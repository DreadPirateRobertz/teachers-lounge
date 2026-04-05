'use client'

import { type KeyboardEvent, type MouseEvent, useEffect, useRef, useState, useCallback } from 'react'

// ── Types ─────────────────────────────────────────────────────────────────────

interface Atom {
  id: string
  symbol: string
  x: number
  y: number
  valence: number  // max bonds this atom can form
}

interface Bond {
  id: string
  fromId: string
  toId: string
  order: 1 | 2 | 3  // single, double, triple
}

interface Props {
  /** Called when the student clicks "Submit". Receives the generated SMILES string. */
  onSubmit: (smiles: string) => void
  /** Optional hint shown above the canvas (e.g. "Draw the structure of benzene"). */
  hint?: string
}

// ── Atom palette ──────────────────────────────────────────────────────────────

const ATOM_PALETTE: Array<{ symbol: string; color: string; valence: number }> = [
  { symbol: 'C', color: '#a8a8a8', valence: 4 },
  { symbol: 'H', color: '#ffffff', valence: 1 },
  { symbol: 'O', color: '#e05252', valence: 2 },
  { symbol: 'N', color: '#5252e0', valence: 3 },
  { symbol: 'S', color: '#e0c852', valence: 2 },
  { symbol: 'P', color: '#e07a52', valence: 5 },
  { symbol: 'F', color: '#52c852', valence: 1 },
  { symbol: 'Cl', color: '#52c852', valence: 1 },
]

const ATOM_COLOR: Record<string, string> = Object.fromEntries(
  ATOM_PALETTE.map((a) => [a.symbol, a.color]),
)
const ATOM_VALENCE: Record<string, number> = Object.fromEntries(
  ATOM_PALETTE.map((a) => [a.symbol, a.valence]),
)

// ── SMILES generation ─────────────────────────────────────────────────────────

/**
 * Generate a SMILES string from the canvas atom/bond graph.
 *
 * Uses a depth-first traversal from the first atom.  Ring closures are
 * detected by back-edges in the DFS tree and annotated with ring numbers.
 * Bond orders above 1 are written as `=` (double) or `#` (triple).
 *
 * Limitations: stereochemistry and aromaticity are not handled — this is
 * sufficient for the Phase 6 structural comparison (canonical normalisation
 * happens server-side via rdkit).
 */
export function generateSmiles(atoms: Atom[], bonds: Bond[]): string {
  if (atoms.length === 0) return ''

  // Build adjacency list: atomId → [{neighbourId, bond}]
  const adj = new Map<string, Array<{ id: string; bond: Bond }>>()
  for (const atom of atoms) adj.set(atom.id, [])
  for (const bond of bonds) {
    adj.get(bond.fromId)?.push({ id: bond.toId, bond })
    adj.get(bond.toId)?.push({ id: bond.fromId, bond })
  }

  const atomById = new Map(atoms.map((a) => [a.id, a]))
  const visited = new Set<string>()
  let ringCounter = 1
  const ringMap = new Map<string, number>()  // edgeKey → ring number

  function edgeKey(a: string, b: string): string {
    return a < b ? `${a}:${b}` : `${b}:${a}`
  }

  function bondChar(order: 1 | 2 | 3): string {
    if (order === 2) return '='
    if (order === 3) return '#'
    return ''
  }

  function dfs(id: string, parentId: string | null): string {
    visited.add(id)
    const atom = atomById.get(id)!
    let result = atom.symbol

    const neighbours = adj.get(id) ?? []
    const branches: string[] = []

    for (const { id: nid, bond } of neighbours) {
      if (nid === parentId) continue

      if (visited.has(nid)) {
        // Back-edge → ring closure
        const key = edgeKey(id, nid)
        if (!ringMap.has(key)) {
          const num = ringCounter++
          ringMap.set(key, num)
          result += `${bondChar(bond.order)}${num}`
        }
      } else {
        branches.push(`${bondChar(bond.order)}${dfs(nid, id)}`)
      }
    }

    if (branches.length === 0) return result
    // Last branch is appended inline; prior branches are wrapped in ()
    for (let i = 0; i < branches.length - 1; i++) {
      result += `(${branches[i]})`
    }
    result += branches[branches.length - 1]
    return result
  }

  return dfs(atoms[0].id, null)
}

// ── Canvas renderer ───────────────────────────────────────────────────────────

const ATOM_RADIUS = 18
const CANVAS_W = 460
const CANVAS_H = 320

function drawScene(
  ctx: CanvasRenderingContext2D,
  atoms: Atom[],
  bonds: Bond[],
  selectedId: string | null,
  draggingId: string | null,
) {
  ctx.clearRect(0, 0, CANVAS_W, CANVAS_H)

  // Background grid
  ctx.strokeStyle = 'rgba(255,255,255,0.04)'
  ctx.lineWidth = 1
  for (let x = 0; x < CANVAS_W; x += 30) {
    ctx.beginPath(); ctx.moveTo(x, 0); ctx.lineTo(x, CANVAS_H); ctx.stroke()
  }
  for (let y = 0; y < CANVAS_H; y += 30) {
    ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(CANVAS_W, y); ctx.stroke()
  }

  const atomById = new Map(atoms.map((a) => [a.id, a]))

  // Draw bonds
  for (const bond of bonds) {
    const a = atomById.get(bond.fromId)
    const b = atomById.get(bond.toId)
    if (!a || !b) continue

    const dx = b.x - a.x
    const dy = b.y - a.y
    const len = Math.hypot(dx, dy) || 1
    const nx = -dy / len  // normal x
    const ny = dx / len   // normal y

    const offsets = bond.order === 1 ? [0] : bond.order === 2 ? [-3, 3] : [-4, 0, 4]

    ctx.strokeStyle = '#6b7280'
    ctx.lineWidth = 2

    for (const off of offsets) {
      ctx.beginPath()
      ctx.moveTo(a.x + nx * off, a.y + ny * off)
      ctx.lineTo(b.x + nx * off, b.y + ny * off)
      ctx.stroke()
    }
  }

  // Draw atoms
  for (const atom of atoms) {
    const isSelected = atom.id === selectedId || atom.id === draggingId
    const color = ATOM_COLOR[atom.symbol] ?? '#888888'

    // Shadow / glow for selected
    if (isSelected) {
      ctx.save()
      ctx.shadowColor = '#60a5fa'
      ctx.shadowBlur = 12
    }

    ctx.fillStyle = color
    ctx.beginPath()
    ctx.arc(atom.x, atom.y, ATOM_RADIUS, 0, Math.PI * 2)
    ctx.fill()

    ctx.strokeStyle = isSelected ? '#60a5fa' : 'rgba(255,255,255,0.2)'
    ctx.lineWidth = isSelected ? 2 : 1
    ctx.stroke()

    if (isSelected) ctx.restore()

    // Label
    ctx.fillStyle = color === '#ffffff' ? '#111' : '#fff'
    ctx.font = `bold ${atom.symbol.length > 1 ? 10 : 12}px monospace`
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText(atom.symbol, atom.x, atom.y)
  }
}

// ── MoleculeBuilder ───────────────────────────────────────────────────────────

let _idCounter = 0
function newId() { return `atom-${++_idCounter}` }
function newBondId() { return `bond-${++_idCounter}` }

/**
 * Interactive molecule canvas for kinesthetic learners.
 *
 * Controls:
 * - Click atom palette then click canvas → place atom
 * - Drag atom → move it
 * - Click two atoms → form a bond between them
 * - Click bond → cycle bond order (1→2→3→1)
 * - Delete key → remove selected atom (and its bonds)
 * - Submit button → generate SMILES and call onSubmit
 */
export default function MoleculeBuilder({ onSubmit, hint }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [atoms, setAtoms] = useState<Atom[]>([])
  const [bonds, setBonds] = useState<Bond[]>([])
  const [selectedAtom, setSelectedAtom] = useState<string | null>(null)
  const [bondSource, setBondSource] = useState<string | null>(null)
  const [dragging, setDragging] = useState<string | null>(null)
  const [dragOffset, setDragOffset] = useState<{ x: number; y: number }>({ x: 0, y: 0 })
  const [activePalette, setActivePalette] = useState<string>('C')
  const [mode, setMode] = useState<'place' | 'bond' | 'select'>('select')

  // Render on state change
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    drawScene(ctx, atoms, bonds, selectedAtom, dragging)
  }, [atoms, bonds, selectedAtom, dragging])

  const getCanvasPos = useCallback((e: MouseEvent<HTMLCanvasElement>) => {
    const rect = canvasRef.current!.getBoundingClientRect()
    const scaleX = CANVAS_W / rect.width
    const scaleY = CANVAS_H / rect.height
    return {
      x: (e.clientX - rect.left) * scaleX,
      y: (e.clientY - rect.top) * scaleY,
    }
  }, [])

  const hitAtom = useCallback((x: number, y: number, excludeId?: string): Atom | null => {
    for (const atom of [...atoms].reverse()) {
      if (atom.id === excludeId) continue
      if (Math.hypot(atom.x - x, atom.y - y) < ATOM_RADIUS) return atom
    }
    return null
  }, [atoms])

  const handleMouseDown = useCallback((e: MouseEvent<HTMLCanvasElement>) => {
    const pos = getCanvasPos(e)
    const hit = hitAtom(pos.x, pos.y)

    if (mode === 'place' && !hit) {
      // Place new atom
      const id = newId()
      setAtoms((prev) => [...prev, {
        id,
        symbol: activePalette,
        x: pos.x,
        y: pos.y,
        valence: ATOM_VALENCE[activePalette] ?? 4,
      }])
      return
    }

    if (mode === 'bond') {
      if (!hit) return
      if (!bondSource) {
        setBondSource(hit.id)
        setSelectedAtom(hit.id)
      } else if (bondSource !== hit.id) {
        // Create or cycle bond between bondSource and hit
        const existing = bonds.find(
          (b) =>
            (b.fromId === bondSource && b.toId === hit.id) ||
            (b.fromId === hit.id && b.toId === bondSource),
        )
        if (existing) {
          setBonds((prev) =>
            prev.map((b) =>
              b.id === existing.id
                ? { ...b, order: ((existing.order % 3) + 1) as 1 | 2 | 3 }
                : b,
            ),
          )
        } else {
          setBonds((prev) => [...prev, {
            id: newBondId(),
            fromId: bondSource,
            toId: hit.id,
            order: 1,
          }])
        }
        setBondSource(null)
        setSelectedAtom(null)
      } else {
        // Clicked same atom → deselect
        setBondSource(null)
        setSelectedAtom(null)
      }
      return
    }

    // select mode
    if (hit) {
      setSelectedAtom(hit.id)
      setDragging(hit.id)
      setDragOffset({ x: pos.x - hit.x, y: pos.y - hit.y })
    } else {
      setSelectedAtom(null)
    }
  }, [mode, activePalette, bondSource, bonds, getCanvasPos, hitAtom])

  const handleMouseMove = useCallback((e: MouseEvent<HTMLCanvasElement>) => {
    if (!dragging) return
    const pos = getCanvasPos(e)
    setAtoms((prev) =>
      prev.map((a) =>
        a.id === dragging
          ? { ...a, x: pos.x - dragOffset.x, y: pos.y - dragOffset.y }
          : a,
      ),
    )
  }, [dragging, dragOffset, getCanvasPos])

  const handleMouseUp = useCallback(() => {
    setDragging(null)
  }, [])

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if ((e.key === 'Delete' || e.key === 'Backspace') && selectedAtom) {
      setAtoms((prev: Atom[]) => prev.filter((a: Atom) => a.id !== selectedAtom))
      setBonds((prev: Bond[]) =>
        prev.filter((b: Bond) => b.fromId !== selectedAtom && b.toId !== selectedAtom),
      )
      setSelectedAtom(null)
    }
  }, [selectedAtom])

  const handleSubmit = useCallback(() => {
    const smiles = generateSmiles(atoms, bonds)
    onSubmit(smiles)
  }, [atoms, bonds, onSubmit])

  const handleClear = useCallback(() => {
    setAtoms([])
    setBonds([])
    setSelectedAtom(null)
    setBondSource(null)
  }, [])

  return (
    <div
      className="flex flex-col gap-2 p-3 bg-bg-card border border-border-mid rounded-xl"
      onKeyDown={handleKeyDown}
      // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
      tabIndex={0}
      aria-label="Molecule builder canvas"
    >
      {hint && (
        <p className="text-xs text-text-dim italic">{hint}</p>
      )}

      {/* Mode toolbar */}
      <div className="flex gap-2 items-center flex-wrap">
        {(['select', 'place', 'bond'] as const).map((m) => (
          <button
            key={m}
            onClick={() => { setMode(m); setBondSource(null) }}
            className={`px-2 py-1 text-[11px] font-mono rounded border transition-colors ${
              mode === m
                ? 'bg-neon-blue/20 border-neon-blue text-neon-blue'
                : 'border-border-dim text-text-dim hover:border-border-mid'
            }`}
            aria-pressed={mode === m}
          >
            {m === 'select' ? '↖ Select' : m === 'place' ? '＋ Place' : '— Bond'}
          </button>
        ))}

        <div className="w-px h-4 bg-border-dim mx-1" />

        {/* Atom palette */}
        {ATOM_PALETTE.map((a) => (
          <button
            key={a.symbol}
            onClick={() => { setActivePalette(a.symbol); setMode('place') }}
            style={{ borderColor: activePalette === a.symbol && mode === 'place' ? a.color : undefined }}
            className={`w-7 h-7 rounded-full text-[10px] font-bold border transition-all ${
              activePalette === a.symbol && mode === 'place'
                ? 'opacity-100 scale-110'
                : 'opacity-60 hover:opacity-80 border-border-dim'
            }`}
            aria-label={`Place ${a.symbol} atom`}
            aria-pressed={activePalette === a.symbol && mode === 'place'}
          >
            <span style={{ color: a.color }}>{a.symbol}</span>
          </button>
        ))}
      </div>

      {/* Canvas */}
      <canvas
        ref={canvasRef}
        width={CANVAS_W}
        height={CANVAS_H}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        className="w-full rounded-lg bg-bg-deep border border-border-dim cursor-crosshair"
        aria-label="Molecule drawing canvas"
        role="img"
      />

      {/* Status bar */}
      <div className="flex gap-3 text-[10px] font-mono text-text-dim">
        <span>{atoms.length} atoms</span>
        <span>{bonds.length} bonds</span>
        {bondSource && (
          <span className="text-neon-blue animate-pulse">
            bonding from {atoms.find((a) => a.id === bondSource)?.symbol ?? '?'} — click target atom
          </span>
        )}
        {selectedAtom && !bondSource && mode === 'select' && (
          <span className="text-text-dim">Del to remove selected atom</span>
        )}
      </div>

      {/* Action buttons */}
      <div className="flex gap-2 justify-end">
        <button
          onClick={handleClear}
          className="px-3 py-1.5 text-xs border border-border-dim text-text-dim rounded hover:border-border-mid transition-colors"
        >
          Clear
        </button>
        <button
          onClick={handleSubmit}
          disabled={atoms.length === 0}
          className="px-3 py-1.5 text-xs bg-neon-blue/20 border border-neon-blue/50 text-neon-blue rounded hover:bg-neon-blue/30 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          Submit Structure
        </button>
      </div>
    </div>
  )
}
