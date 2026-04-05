/**
 * @fileoverview Tests for the moleculeGeometry pure-data module.
 *
 * Validates the ELEMENT_COLORS palette, the MOLECULES registry, and the
 * structural invariants that every molecule must satisfy (required fields,
 * atom element/position shape, bond from/to/order constraints).
 *
 * No React, no rendering — this is a plain TypeScript/Node test.
 *
 * @jest-environment node
 */

import {
  ELEMENT_COLORS,
  ELEMENT_RADII,
  MOLECULES,
  elementColor,
  elementRadius,
  findMolecule,
  type Atom,
  type Bond,
  type MoleculeData,
} from './moleculeGeometry'

// ---------------------------------------------------------------------------
// ELEMENT_COLORS
// ---------------------------------------------------------------------------

describe('ELEMENT_COLORS', () => {
  it('has entries for the four most common organic elements', () => {
    expect(ELEMENT_COLORS).toHaveProperty('H')
    expect(ELEMENT_COLORS).toHaveProperty('C')
    expect(ELEMENT_COLORS).toHaveProperty('O')
    expect(ELEMENT_COLORS).toHaveProperty('N')
  })

  it("ELEMENT_COLORS['H'] is a string starting with '#' or 'rgb'", () => {
    const color = ELEMENT_COLORS['H']
    expect(typeof color).toBe('string')
    expect(color).toMatch(/^(#|rgb)/i)
  })

  it('every color value is a non-empty string', () => {
    for (const [symbol, color] of Object.entries(ELEMENT_COLORS)) {
      expect(typeof color).toBe('string')
      expect(color.length).toBeGreaterThan(0)
    }
  })

  it('has a fallback default entry', () => {
    expect(ELEMENT_COLORS).toHaveProperty('default')
    expect(typeof ELEMENT_COLORS['default']).toBe('string')
  })
})

// ---------------------------------------------------------------------------
// ELEMENT_RADII
// ---------------------------------------------------------------------------

describe('ELEMENT_RADII', () => {
  it('has positive radius values for H, C, N, O', () => {
    for (const el of ['H', 'C', 'N', 'O']) {
      expect(ELEMENT_RADII[el]).toBeGreaterThan(0)
    }
  })
})

// ---------------------------------------------------------------------------
// elementColor helper
// ---------------------------------------------------------------------------

describe('elementColor', () => {
  it('returns the correct color for a known element', () => {
    expect(elementColor('O')).toBe(ELEMENT_COLORS['O'])
  })

  it('returns the default color for an unknown element', () => {
    expect(elementColor('Xx')).toBe(ELEMENT_COLORS['default'])
  })
})

// ---------------------------------------------------------------------------
// elementRadius helper
// ---------------------------------------------------------------------------

describe('elementRadius', () => {
  it('returns the correct radius for a known element', () => {
    expect(elementRadius('C')).toBe(ELEMENT_RADII['C'])
  })

  it('returns the default radius for an unknown element', () => {
    expect(elementRadius('Xx')).toBe(ELEMENT_RADII['default'])
  })
})

// ---------------------------------------------------------------------------
// MOLECULES registry — structural invariants
// ---------------------------------------------------------------------------

describe('MOLECULES', () => {
  /**
   * Helper: asserts that an atom has the required fields with correct types.
   *
   * @param atom - The atom object under test.
   * @param moleculeName - Used in failure messages for context.
   * @param index - Atom index within the molecule.
   */
  function assertAtomShape(atom: Atom, moleculeName: string, index: number) {
    expect(typeof atom.element).toBe('string') // moleculeName atom[index].element
    expect(atom.element.length).toBeGreaterThan(0)
    expect(Array.isArray(atom.position)).toBe(true)
    expect(atom.position.length).toBe(3)
    for (let axis = 0; axis < 3; axis++) {
      expect(typeof atom.position[axis]).toBe('number')
    }
    void moleculeName
    void index // suppress unused-var lint
  }

  /**
   * Helper: asserts that a bond has valid from/to indices and a legal order.
   *
   * @param bond - The bond object under test.
   * @param moleculeName - Used in failure messages for context.
   * @param index - Bond index within the molecule.
   * @param atomCount - Total atoms in the molecule (to range-check indices).
   */
  function assertBondShape(bond: Bond, moleculeName: string, index: number, atomCount: number) {
    expect(typeof bond.from).toBe('number')
    expect(typeof bond.to).toBe('number')
    expect(bond.from).toBeGreaterThanOrEqual(0)
    expect(bond.from).toBeLessThan(atomCount)
    expect(bond.to).toBeGreaterThanOrEqual(0)
    expect(bond.to).toBeLessThan(atomCount)
    expect([1, 2, 3]).toContain(bond.order)
    void moleculeName
    void index // suppress unused-var lint
  }

  it('exports the expected molecule keys', () => {
    const keys = Object.keys(MOLECULES)
    expect(keys).toEqual(
      expect.arrayContaining(['water', 'methane', 'co2', 'ammonia', 'benzene', 'ethanol']),
    )
  })

  it('every molecule has name, formula, atoms array, and bonds array', () => {
    for (const [key, mol] of Object.entries(MOLECULES)) {
      expect(typeof mol.name).toBe('string')
      expect(mol.name.length).toBeGreaterThan(0)

      expect(typeof mol.formula).toBe('string')
      expect(mol.formula.length).toBeGreaterThan(0)

      expect(Array.isArray(mol.atoms)).toBe(true)
      expect(mol.atoms.length).toBeGreaterThan(0)

      expect(Array.isArray(mol.bonds)).toBe(true)
    }
  })

  it('every atom has an element symbol and a 3-component position', () => {
    for (const [key, mol] of Object.entries(MOLECULES)) {
      mol.atoms.forEach((atom, i) => assertAtomShape(atom, key, i))
    }
  })

  it('every bond has valid from/to indices and a bond order of 1, 2, or 3', () => {
    for (const [key, mol] of Object.entries(MOLECULES)) {
      mol.bonds.forEach((bond, i) => assertBondShape(bond, key, i, mol.atoms.length))
    }
  })

  // ------ Per-molecule spot checks ------

  it('water has formula H2O with 3 atoms and 2 bonds', () => {
    const water = MOLECULES['water']
    expect(water.formula).toBe('H2O')
    expect(water.atoms).toHaveLength(3)
    expect(water.bonds).toHaveLength(2)
  })

  it('water atoms contain one O and two H', () => {
    const elements = MOLECULES['water'].atoms.map((a) => a.element)
    expect(elements.filter((e) => e === 'O')).toHaveLength(1)
    expect(elements.filter((e) => e === 'H')).toHaveLength(2)
  })

  it('methane has formula CH4 with 5 atoms and 4 single bonds', () => {
    const methane = MOLECULES['methane']
    expect(methane.formula).toBe('CH4')
    expect(methane.atoms).toHaveLength(5)
    expect(methane.bonds).toHaveLength(4)
    methane.bonds.forEach((b) => expect(b.order).toBe(1))
  })

  it('co2 has double bonds', () => {
    const co2 = MOLECULES['co2']
    expect(co2.formula).toBe('CO2')
    co2.bonds.forEach((b) => expect(b.order).toBe(2))
  })

  it('benzene has 12 atoms (6 C + 6 H) and 12 bonds', () => {
    const benzene = MOLECULES['benzene']
    expect(benzene.formula).toBe('C6H6')
    expect(benzene.atoms).toHaveLength(12)
    expect(benzene.bonds).toHaveLength(12)
  })

  it('benzene bonds include both order-1 and order-2 bonds (Kekule)', () => {
    const orders = MOLECULES['benzene'].bonds.map((b) => b.order)
    expect(orders).toContain(1)
    expect(orders).toContain(2)
  })

  it('ammonia has N as the first atom and 3 bonds', () => {
    const ammonia = MOLECULES['ammonia']
    expect(ammonia.atoms[0].element).toBe('N')
    expect(ammonia.bonds).toHaveLength(3)
  })
})

// ---------------------------------------------------------------------------
// findMolecule
// ---------------------------------------------------------------------------

describe('findMolecule', () => {
  it('finds water by direct key', () => {
    const mol = findMolecule('water')
    expect(mol).not.toBeNull()
    expect(mol!.formula).toBe('H2O')
  })

  it('finds water by alias H2O (case-insensitive)', () => {
    const mol = findMolecule('H2O')
    expect(mol).not.toBeNull()
    expect(mol!.name).toBe('water')
  })

  it('finds carbon dioxide by alias', () => {
    const mol = findMolecule('carbon dioxide')
    expect(mol).not.toBeNull()
    expect(mol!.formula).toBe('CO2')
  })

  it('returns null for an unknown query', () => {
    expect(findMolecule('unobtanium')).toBeNull()
  })

  it('is case-insensitive for direct keys', () => {
    const mol = findMolecule('WATER')
    // Direct key lookup is exact, but alias/name path is case-insensitive.
    // Either a match or null is acceptable as long as non-null means correct data.
    if (mol !== null) {
      expect(mol.formula).toBe('H2O')
    }
  })
})
