/**
 * @jest-environment jsdom
 *
 * Tests for MoleculeViewer and moleculeGeometry utilities.
 * Three.js / React Three Fiber are mocked so tests run in jsdom without WebGL.
 */
import React from 'react'
import { render, screen } from '@testing-library/react'

// ── Mock Three.js / R3F so tests run in jsdom without WebGL ─────────────────

jest.mock('@react-three/fiber', () => ({
  Canvas: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="r3f-canvas">{children}</div>
  ),
  useFrame: jest.fn(),
  useThree: jest.fn(() => ({ camera: {}, gl: {} })),
}))

jest.mock('@react-three/drei', () => ({
  OrbitControls: () => null,
  Sphere: ({ children, ...props }: React.PropsWithChildren<Record<string, unknown>>) => (
    <mesh data-testid="atom-sphere" {...(props as object)}>
      {children}
    </mesh>
  ),
  Cylinder: (props: Record<string, unknown>) => (
    <mesh data-testid="bond-cylinder" {...(props as object)} />
  ),
  Html: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="drei-html">{children}</div>
  ),
}))

// Mock next/dynamic so MoleculeViewerCanvas renders synchronously.
// MoleculeViewer calls dynamic(() => import('./MoleculeViewerCanvas'), {ssr:false}).
// We intercept and require MoleculeViewerCanvas directly.
jest.mock('next/dynamic', () => {
  const MoleculeViewerCanvas = require('./MoleculeViewerCanvas').default // jest mock — require is intentional
  return (_loader: unknown, _opts?: unknown) => MoleculeViewerCanvas
})

// ── Import under test (after mocks are set up) ───────────────────────────────

import MoleculeViewer from './MoleculeViewer'
import { findMolecule, MOLECULES } from './moleculeGeometry'

// ── MoleculeViewer component tests ──────────────────────────────────────────

describe('MoleculeViewer', () => {
  it('renders canvas for a known molecule', () => {
    render(<MoleculeViewer molecule="water" />)
    expect(screen.getByTestId('r3f-canvas')).toBeInTheDocument()
  })

  it('renders correct atom count for water (3 atom spheres)', () => {
    render(<MoleculeViewer molecule="water" />)
    const spheres = screen.getAllByTestId('atom-sphere')
    expect(spheres).toHaveLength(3)
  })

  it('renders correct atom count for methane (5 atom spheres)', () => {
    render(<MoleculeViewer molecule="methane" />)
    const spheres = screen.getAllByTestId('atom-sphere')
    expect(spheres).toHaveLength(5)
  })

  it('renders "UNKNOWN MOLECULE" fallback for an unrecognised key', () => {
    render(<MoleculeViewer molecule="xyzzy-not-real" />)
    expect(screen.getByText('UNKNOWN MOLECULE')).toBeInTheDocument()
  })

  it('renders "UNKNOWN MOLECULE" fallback for an empty string', () => {
    render(<MoleculeViewer molecule="" />)
    expect(screen.getByText('UNKNOWN MOLECULE')).toBeInTheDocument()
  })

  it('accepts a formula string H2O and renders the molecule', () => {
    render(<MoleculeViewer molecule="H2O" />)
    expect(screen.getByTestId('r3f-canvas')).toBeInTheDocument()
    expect(screen.getAllByTestId('atom-sphere')).toHaveLength(3)
  })

  it('respects width and height props via inline style', () => {
    const { container } = render(<MoleculeViewer molecule="water" width={500} height={400} />)
    const wrapper = container.firstChild as HTMLElement
    expect(wrapper).not.toBeNull()
    expect(wrapper.style.width).toBe('500px')
    expect(wrapper.style.height).toBe('400px')
  })
})

// ── findMolecule utility tests ───────────────────────────────────────────────

describe('findMolecule', () => {
  it('finds water by formula H2O', () => {
    const mol = findMolecule('H2O')
    expect(mol).not.toBeNull()
    expect(mol!.name).toBe('water')
  })

  it('finds water by formula lowercase h2o', () => {
    const mol = findMolecule('h2o')
    expect(mol).not.toBeNull()
    expect(mol!.formula).toBe('H2O')
  })

  it('finds methane by name', () => {
    const mol = findMolecule('methane')
    expect(mol).not.toBeNull()
    expect(mol!.formula).toBe('CH4')
  })

  it('finds methane by formula CH4', () => {
    const mol = findMolecule('CH4')
    expect(mol).not.toBeNull()
    expect(mol!.name).toBe('methane')
  })

  it('finds co2 by key', () => {
    const mol = findMolecule('co2')
    expect(mol).not.toBeNull()
    expect(mol!.formula).toBe('CO2')
  })

  it('finds benzene by name', () => {
    const mol = findMolecule('benzene')
    expect(mol).not.toBeNull()
    expect(mol!.formula).toBe('C6H6')
  })

  it('finds ethanol by alias "ethyl alcohol"', () => {
    const mol = findMolecule('ethyl alcohol')
    expect(mol).not.toBeNull()
    expect(mol!.name).toBe('ethanol')
  })

  it('returns null for unknown query xyz', () => {
    expect(findMolecule('xyz')).toBeNull()
  })

  it('returns null for empty string', () => {
    expect(findMolecule('')).toBeNull()
  })

  it('MOLECULES contains all six built-in molecules', () => {
    const keys = Object.keys(MOLECULES)
    expect(keys).toContain('water')
    expect(keys).toContain('methane')
    expect(keys).toContain('co2')
    expect(keys).toContain('ammonia')
    expect(keys).toContain('benzene')
    expect(keys).toContain('ethanol')
  })

  it('water molecule has 3 atoms', () => {
    expect(MOLECULES['water'].atoms).toHaveLength(3)
  })

  it('methane molecule has 5 atoms', () => {
    expect(MOLECULES['methane'].atoms).toHaveLength(5)
  })

  it('benzene molecule has 12 atoms (6C + 6H)', () => {
    expect(MOLECULES['benzene'].atoms).toHaveLength(12)
  })
})
