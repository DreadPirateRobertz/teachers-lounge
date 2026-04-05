/**
 * @jest-environment jsdom
 *
 * Tests for BossCharacter — render, boss name, HP bar ratio.
 * Three.js is fully mocked so no WebGL context is needed.
 */

import React from 'react'
import { render, screen } from '@testing-library/react'
import { BossCharacter } from './BossCharacter'
import type { BossId } from './types'

// ---------------------------------------------------------------------------
// Three.js mock
// ---------------------------------------------------------------------------

jest.mock('three', () => ({
  WebGLRenderer: jest.fn(() => ({
    setSize: jest.fn(),
    setPixelRatio: jest.fn(),
    render: jest.fn(),
    dispose: jest.fn(),
    domElement: document.createElement('canvas'),
  })),
  Scene: jest.fn(() => ({ add: jest.fn(), background: null })),
  PerspectiveCamera: jest.fn(() => ({
    position: { set: jest.fn() },
    aspect: 0,
    updateProjectionMatrix: jest.fn(),
  })),
  AmbientLight: jest.fn(() => ({})),
  PointLight: jest.fn(() => ({ position: { set: jest.fn() } })),
  Group: jest.fn(() => ({
    add: jest.fn(),
    rotation: { x: 0, y: 0, z: 0 },
    position: { set: jest.fn() },
    scale: { setScalar: jest.fn() },
  })),
  IcosahedronGeometry: jest.fn(() => ({})),
  TorusGeometry: jest.fn(() => ({})),
  SphereGeometry: jest.fn(() => ({})),
  CylinderGeometry: jest.fn(() => ({})),
  BoxGeometry: jest.fn(() => ({})),
  TetrahedronGeometry: jest.fn(() => ({})),
  MeshPhongMaterial: jest.fn(() => ({})),
  Mesh: jest.fn(() => ({
    rotation: { x: 0, y: 0, z: 0 },
    position: { set: jest.fn() },
  })),
  Color: jest.fn(function (this: Record<string, unknown>, _c: unknown) {
    this._color = _c
  }),
}))

// Stub rAF/cAF so Three.js animation loop doesn't error in jsdom
beforeEach(() => {
  jest.spyOn(globalThis, 'requestAnimationFrame').mockReturnValue(1)
  jest.spyOn(globalThis, 'cancelAnimationFrame').mockImplementation(() => undefined)
})

afterEach(() => {
  jest.restoreAllMocks()
})

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

const ALL_BOSS_IDS: BossId[] = [
  'the_atom',
  'bonding_brothers',
  'name_lord',
  'the_stereochemist',
  'the_reactor',
]

const BOSS_NAMES: Record<BossId, string> = {
  the_atom: 'THE ATOM',
  bonding_brothers: 'BONDING BROTHERS',
  name_lord: 'NAME LORD',
  the_stereochemist: 'THE STEREOCHEMIST',
  the_reactor: 'THE REACTOR',
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('BossCharacter', () => {
  it('renders without throwing for the_atom', () => {
    expect(() =>
      render(<BossCharacter bossId="the_atom" hp={100} maxHp={100} />),
    ).not.toThrow()
  })

  it.each(ALL_BOSS_IDS)('renders without throwing for %s', (bossId) => {
    expect(() =>
      render(<BossCharacter bossId={bossId} hp={75} maxHp={100} />),
    ).not.toThrow()
  })

  it.each(ALL_BOSS_IDS)('shows the correct boss name for %s', (bossId) => {
    render(<BossCharacter bossId={bossId} hp={50} maxHp={100} />)
    expect(screen.getByTestId('boss-name').textContent).toBe(BOSS_NAMES[bossId])
  })

  describe('HP bar ratio', () => {
    it('HP bar is full (100%) at max HP', () => {
      render(<BossCharacter bossId="the_atom" hp={100} maxHp={100} />)
      const bar = screen.getByTestId('hp-bar-fill')
      expect(bar.style.width).toBe('100%')
    })

    it('HP bar is empty (0%) at 0 HP', () => {
      render(<BossCharacter bossId="the_atom" hp={0} maxHp={100} />)
      const bar = screen.getByTestId('hp-bar-fill')
      expect(bar.style.width).toBe('0%')
    })

    it('HP bar is ~50% at half HP', () => {
      render(<BossCharacter bossId="the_atom" hp={50} maxHp={100} />)
      const bar = screen.getByTestId('hp-bar-fill')
      expect(bar.style.width).toBe('50%')
    })

    it('HP bar is ~25% at quarter HP', () => {
      render(<BossCharacter bossId="the_atom" hp={25} maxHp={100} />)
      const bar = screen.getByTestId('hp-bar-fill')
      expect(bar.style.width).toBe('25%')
    })

    it('HP bar reflects hp/maxHp ratio correctly', () => {
      render(<BossCharacter bossId="the_reactor" hp={60} maxHp={200} />)
      const bar = screen.getByTestId('hp-bar-fill')
      // 60/200 = 0.3 = 30%
      expect(bar.style.width).toBe('30%')
    })

    it('HP bar clamps to 0% when hp is negative', () => {
      render(<BossCharacter bossId="the_atom" hp={-10} maxHp={100} />)
      const bar = screen.getByTestId('hp-bar-fill')
      expect(bar.style.width).toBe('0%')
    })

    it('HP bar clamps to 100% when hp exceeds maxHp', () => {
      render(<BossCharacter bossId="the_atom" hp={150} maxHp={100} />)
      const bar = screen.getByTestId('hp-bar-fill')
      expect(bar.style.width).toBe('100%')
    })
  })
})
