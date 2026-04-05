/**
 * Unit tests for SMILES generation from the MoleculeBuilder canvas state.
 * Tests the pure `generateSmiles` function — no browser rendering required.
 */
import { generateSmiles } from './MoleculeBuilder'

// ── Fixtures ──────────────────────────────────────────────────────────────────

function atom(
  id: string,
  symbol: string,
): { id: string; symbol: string; x: number; y: number; valence: number } {
  return { id, symbol, x: 0, y: 0, valence: 4 }
}

function bond(id: string, fromId: string, toId: string, order: 1 | 2 | 3 = 1) {
  return { id, fromId, toId, order }
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('generateSmiles', () => {
  it('returns empty string for empty atom list', () => {
    expect(generateSmiles([], [])).toBe('')
  })

  it('returns single atom symbol for lone atom', () => {
    expect(generateSmiles([atom('a1', 'C')], [])).toBe('C')
  })

  it('generates methane (CH4) structure', () => {
    const atoms = [
      atom('c', 'C'),
      atom('h1', 'H'),
      atom('h2', 'H'),
      atom('h3', 'H'),
      atom('h4', 'H'),
    ]
    const bonds = [
      bond('b1', 'c', 'h1'),
      bond('b2', 'c', 'h2'),
      bond('b3', 'c', 'h3'),
      bond('b4', 'c', 'h4'),
    ]
    const smiles = generateSmiles(atoms, bonds)
    // SMILES should start with C and contain 4 H atoms
    expect(smiles).toMatch(/^C/)
    expect((smiles.match(/H/g) ?? []).length).toBe(4)
  })

  it('generates ethanol structure containing O', () => {
    const atoms = [atom('c1', 'C'), atom('c2', 'C'), atom('o', 'O')]
    const bonds = [bond('b1', 'c1', 'c2'), bond('b2', 'c2', 'o')]
    const smiles = generateSmiles(atoms, bonds)
    expect(smiles).toContain('C')
    expect(smiles).toContain('O')
  })

  it('includes = for double bond', () => {
    const atoms = [atom('c1', 'C'), atom('c2', 'C')]
    const bonds = [bond('b1', 'c1', 'c2', 2)]
    const smiles = generateSmiles(atoms, bonds)
    expect(smiles).toContain('=')
  })

  it('includes # for triple bond', () => {
    const atoms = [atom('c1', 'C'), atom('n', 'N')]
    const bonds = [bond('b1', 'c1', 'n', 3)]
    const smiles = generateSmiles(atoms, bonds)
    expect(smiles).toContain('#')
  })

  it('handles ring closure with ring number annotation', () => {
    // 3-membered ring: C-C-C (cyclopropane)
    const atoms = [atom('a', 'C'), atom('b', 'C'), atom('c', 'C')]
    const bonds = [
      bond('b1', 'a', 'b'),
      bond('b2', 'b', 'c'),
      bond('b3', 'c', 'a'), // back-edge
    ]
    const smiles = generateSmiles(atoms, bonds)
    // Ring closure should include a digit
    expect(smiles).toMatch(/\d/)
  })

  it('generates linear chain without ring numbers', () => {
    // propane: C-C-C
    const atoms = [atom('a', 'C'), atom('b', 'C'), atom('c', 'C')]
    const bonds = [bond('b1', 'a', 'b'), bond('b2', 'b', 'c')]
    const smiles = generateSmiles(atoms, bonds)
    // Should be CCC (no ring numbers, no parentheses for linear chain)
    expect(smiles).not.toMatch(/\d/)
    expect(smiles).toBe('CCC')
  })

  it('generates branched structure with parentheses', () => {
    // isobutane: C with 3 methyl branches
    const atoms = [atom('center', 'C'), atom('m1', 'C'), atom('m2', 'C'), atom('m3', 'C')]
    const bonds = [
      bond('b1', 'center', 'm1'),
      bond('b2', 'center', 'm2'),
      bond('b3', 'center', 'm3'),
    ]
    const smiles = generateSmiles(atoms, bonds)
    // Branched structure should use parentheses for branches
    expect(smiles).toContain('(')
    expect(smiles).toContain(')')
  })
})
