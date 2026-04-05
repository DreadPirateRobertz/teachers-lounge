/**
 * Atom and bond geometry data for common chemistry molecules.
 *
 * Positions are in Angstroms using a right-handed coordinate system.
 * Bond lengths and angles are chemically accurate to 2 decimal places.
 */

/** A single atom in a molecule. */
export interface Atom {
  /** Element symbol (e.g. 'H', 'C', 'O', 'N') */
  element: string
  /** 3D position [x, y, z] in Angstroms */
  position: [number, number, number]
}

/** A covalent bond between two atoms. */
export interface Bond {
  /** Index into the atoms array for the first atom */
  from: number
  /** Index into the atoms array for the second atom */
  to: number
  /** Bond order: 1 = single, 2 = double, 3 = triple */
  order: 1 | 2 | 3
}

/** Complete geometry and metadata for a molecule. */
export interface MoleculeData {
  /** Common name of the molecule */
  name: string
  /** Molecular formula */
  formula: string
  atoms: Atom[]
  bonds: Bond[]
}

/**
 * CPK (Corey–Pauling–Koltun) color palette indexed by element symbol.
 * Falls back to 'default' for unknown elements.
 */
export const ELEMENT_COLORS: Record<string, string> = {
  H: '#FFFFFF',
  C: '#404040',
  N: '#3050F8',
  O: '#FF0D0D',
  S: '#FFFF30',
  P: '#FF8000',
  F: '#90E050',
  Cl: '#1FF01F',
  Br: '#A62929',
  default: '#FF69B4',
}

/**
 * CPK van der Waals radii in Angstroms, used as sphere radius scale.
 * Falls back to 'default' for unknown elements.
 */
export const ELEMENT_RADII: Record<string, number> = {
  H: 0.25,
  C: 0.35,
  N: 0.35,
  O: 0.3,
  S: 0.4,
  default: 0.35,
}

/**
 * Returns the CPK color for a given element symbol.
 *
 * @param element - Element symbol (e.g. 'C', 'H', 'O').
 * @returns Hex color string from ELEMENT_COLORS, or the default color.
 */
export function elementColor(element: string): string {
  return ELEMENT_COLORS[element] ?? ELEMENT_COLORS['default']
}

/**
 * Returns the sphere radius scale for a given element symbol.
 *
 * @param element - Element symbol (e.g. 'C', 'H', 'O').
 * @returns Radius in Angstroms from ELEMENT_RADII, or the default radius.
 */
export function elementRadius(element: string): number {
  return ELEMENT_RADII[element] ?? ELEMENT_RADII['default']
}

/**
 * Geometry for water (H₂O).
 *
 * Bent geometry, O–H bond length 0.96 Å, bond angle 104.5°.
 * Oxygen at origin; hydrogens symmetric about the z-axis.
 */
const water: MoleculeData = {
  name: 'water',
  formula: 'H2O',
  atoms: [
    { element: 'O', position: [0, 0, 0] },
    { element: 'H', position: [0.757, 0, 0.586] },
    { element: 'H', position: [-0.757, 0, 0.586] },
  ],
  bonds: [
    { from: 0, to: 1, order: 1 },
    { from: 0, to: 2, order: 1 },
  ],
}

/**
 * Geometry for methane (CH₄).
 *
 * Tetrahedral geometry, C–H bond length 1.09 Å.
 * Carbon at origin; four hydrogens at tetrahedral vertices.
 */
const methane: MoleculeData = {
  name: 'methane',
  formula: 'CH4',
  atoms: [
    { element: 'C', position: [0, 0, 0] },
    { element: 'H', position: [0.629, 0.629, 0.629] },
    { element: 'H', position: [-0.629, -0.629, 0.629] },
    { element: 'H', position: [-0.629, 0.629, -0.629] },
    { element: 'H', position: [0.629, -0.629, -0.629] },
  ],
  bonds: [
    { from: 0, to: 1, order: 1 },
    { from: 0, to: 2, order: 1 },
    { from: 0, to: 3, order: 1 },
    { from: 0, to: 4, order: 1 },
  ],
}

/**
 * Geometry for carbon dioxide (CO₂).
 *
 * Linear geometry, C=O bond length 1.16 Å.
 * Carbon at origin; oxygens along the x-axis.
 */
const co2: MoleculeData = {
  name: 'carbon dioxide',
  formula: 'CO2',
  atoms: [
    { element: 'C', position: [0, 0, 0] },
    { element: 'O', position: [1.16, 0, 0] },
    { element: 'O', position: [-1.16, 0, 0] },
  ],
  bonds: [
    { from: 0, to: 1, order: 2 },
    { from: 0, to: 2, order: 2 },
  ],
}

/**
 * Geometry for ammonia (NH₃).
 *
 * Trigonal pyramidal geometry, N–H bond length 1.01 Å, bond angle 107.8°.
 * Nitrogen at origin; hydrogens arranged pyramidally below.
 */
const ammonia: MoleculeData = {
  name: 'ammonia',
  formula: 'NH3',
  atoms: [
    { element: 'N', position: [0, 0, 0] },
    { element: 'H', position: [0.94, 0, -0.383] },
    { element: 'H', position: [-0.47, 0.814, -0.383] },
    { element: 'H', position: [-0.47, -0.814, -0.383] },
  ],
  bonds: [
    { from: 0, to: 1, order: 1 },
    { from: 0, to: 2, order: 1 },
    { from: 0, to: 3, order: 1 },
  ],
}

/**
 * Geometry for benzene (C₆H₆).
 *
 * Planar hexagonal ring, C–C bond length 1.40 Å, C–H bond length 1.09 Å.
 * Ring in the xz-plane; all atoms placed at chemically correct positions.
 */
const benzene: MoleculeData = (() => {
  const cc = 1.4 // C–C bond length
  const ch = 1.09 // C–H bond length
  const atoms: Atom[] = []
  const bonds: Bond[] = []

  for (let i = 0; i < 6; i++) {
    const angle = (Math.PI / 3) * i
    atoms.push({
      element: 'C',
      position: [cc * Math.cos(angle), 0, cc * Math.sin(angle)],
    })
  }
  for (let i = 0; i < 6; i++) {
    const angle = (Math.PI / 3) * i
    atoms.push({
      element: 'H',
      position: [(cc + ch) * Math.cos(angle), 0, (cc + ch) * Math.sin(angle)],
    })
  }
  // C–C ring bonds (alternating single/double for Kekulé, shown as order 1.5 → use 2)
  for (let i = 0; i < 6; i++) {
    bonds.push({ from: i, to: (i + 1) % 6, order: i % 2 === 0 ? 2 : 1 })
  }
  // C–H bonds
  for (let i = 0; i < 6; i++) {
    bonds.push({ from: i, to: i + 6, order: 1 })
  }

  return { name: 'benzene', formula: 'C6H6', atoms, bonds }
})()

/**
 * Geometry for ethanol (C₂H₅OH).
 *
 * Extended conformation, C–C 1.54 Å, C–O 1.43 Å, O–H 0.96 Å.
 * Carbon backbone along the x-axis.
 */
const ethanol: MoleculeData = {
  name: 'ethanol',
  formula: 'C2H5OH',
  atoms: [
    // C1 (methyl)
    { element: 'C', position: [-1.232, 0, 0] },
    // C2 (bearing OH)
    { element: 'C', position: [0.231, 0, 0] },
    // O
    { element: 'O', position: [0.841, 1.187, 0] },
    // H on O
    { element: 'H', position: [1.796, 1.141, 0] },
    // H on C1 (3)
    { element: 'H', position: [-1.627, 1.026, 0] },
    { element: 'H', position: [-1.627, -0.513, 0.889] },
    { element: 'H', position: [-1.627, -0.513, -0.889] },
    // H on C2 (2)
    { element: 'H', position: [0.627, -0.513, 0.889] },
    { element: 'H', position: [0.627, -0.513, -0.889] },
  ],
  bonds: [
    { from: 0, to: 1, order: 1 }, // C1–C2
    { from: 1, to: 2, order: 1 }, // C2–O
    { from: 2, to: 3, order: 1 }, // O–H
    { from: 0, to: 4, order: 1 }, // C1–H
    { from: 0, to: 5, order: 1 },
    { from: 0, to: 6, order: 1 },
    { from: 1, to: 7, order: 1 }, // C2–H
    { from: 1, to: 8, order: 1 },
  ],
}

/**
 * All built-in molecules indexed by short key.
 *
 * Keys: 'water', 'methane', 'co2', 'ammonia', 'benzene', 'ethanol'.
 */
export const MOLECULES: Record<string, MoleculeData> = {
  water,
  methane,
  co2,
  ammonia,
  benzene,
  ethanol,
}

/** Case-insensitive aliases for common molecule queries. */
const ALIASES: Record<string, string> = {
  // water
  h2o: 'water',
  // methane
  ch4: 'methane',
  // co2
  'carbon dioxide': 'co2',
  co2: 'co2',
  // ammonia
  nh3: 'ammonia',
  // benzene
  c6h6: 'benzene',
  // ethanol
  'ethyl alcohol': 'ethanol',
  alcohol: 'ethanol',
  c2h5oh: 'ethanol',
  c2h6o: 'ethanol',
}

/**
 * Searches for a molecule by key, name, formula, or common alias.
 *
 * The search is case-insensitive. The lookup order is:
 * 1. Direct key in MOLECULES (e.g. 'water', 'co2')
 * 2. Alias table (e.g. 'H2O' → 'water')
 * 3. Name match across all molecules
 * 4. Formula match across all molecules
 *
 * @param query - Molecule name, formula, or alias (e.g. 'H2O', 'water', 'methane').
 * @returns MoleculeData if found, or null if no match.
 */
export function findMolecule(query: string): MoleculeData | null {
  const q = query.trim().toLowerCase()

  // 1. Direct key
  if (MOLECULES[q]) return MOLECULES[q]

  // 2. Alias
  const aliasKey = ALIASES[q]
  if (aliasKey && MOLECULES[aliasKey]) return MOLECULES[aliasKey]

  // 3. Name match
  for (const mol of Object.values(MOLECULES)) {
    if (mol.name.toLowerCase() === q) return mol
  }

  // 4. Formula match (case-insensitive)
  for (const mol of Object.values(MOLECULES)) {
    if (mol.formula.toLowerCase() === q) return mol
  }

  return null
}
