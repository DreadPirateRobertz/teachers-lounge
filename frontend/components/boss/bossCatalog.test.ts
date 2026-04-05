/**
 * Tests for bossCatalog — validates catalog completeness and field integrity.
 */

import { bossCatalog } from './bossCatalog'
import type { BossConfig, BossId, GeometryType } from './types'

const EXPECTED_IDS: BossId[] = [
  'the_atom',
  'bonding_brothers',
  'name_lord',
  'the_stereochemist',
  'the_reactor',
]

const EXPECTED_NAMES: Record<BossId, string> = {
  the_atom: 'THE ATOM',
  bonding_brothers: 'BONDING BROTHERS',
  name_lord: 'NAME LORD',
  the_stereochemist: 'THE STEREOCHEMIST',
  the_reactor: 'THE REACTOR',
}

const VALID_GEOMETRY_TYPES: GeometryType[] = [
  'atom',
  'dumbbell',
  'grid',
  'tetrahedron',
  'flask',
]

describe('bossCatalog', () => {
  it('contains exactly 5 bosses', () => {
    expect(bossCatalog).toHaveLength(5)
  })

  it('contains all 5 expected boss IDs', () => {
    const ids = bossCatalog.map((b) => b.id)
    for (const expectedId of EXPECTED_IDS) {
      expect(ids).toContain(expectedId)
    }
  })

  it('has unique boss IDs', () => {
    const ids = bossCatalog.map((b) => b.id)
    const uniqueIds = new Set(ids)
    expect(uniqueIds.size).toBe(5)
  })

  it('has unique tiers 1–5', () => {
    const tiers = bossCatalog.map((b) => b.tier).sort()
    expect(tiers).toEqual([1, 2, 3, 4, 5])
  })

  it.each(EXPECTED_IDS)(
    'boss %s has all required fields',
    (id) => {
      const boss = bossCatalog.find((b) => b.id === id) as BossConfig
      expect(boss).toBeDefined()
      expect(typeof boss.id).toBe('string')
      expect(typeof boss.name).toBe('string')
      expect(typeof boss.tier).toBe('number')
      expect(typeof boss.topic).toBe('string')
      expect(typeof boss.taunt).toBe('string')
      expect(typeof boss.color).toBe('string')
      expect(typeof boss.accentColor).toBe('string')
      expect(typeof boss.scale).toBe('number')
      expect(typeof boss.geometryType).toBe('string')
    },
  )

  it.each(EXPECTED_IDS)(
    'boss %s name matches expected display name',
    (id) => {
      const boss = bossCatalog.find((b) => b.id === id) as BossConfig
      expect(boss.name).toBe(EXPECTED_NAMES[id])
    },
  )

  it.each(EXPECTED_IDS)(
    'boss %s tier is between 1 and 5',
    (id) => {
      const boss = bossCatalog.find((b) => b.id === id) as BossConfig
      expect(boss.tier).toBeGreaterThanOrEqual(1)
      expect(boss.tier).toBeLessThanOrEqual(5)
    },
  )

  it.each(EXPECTED_IDS)(
    'boss %s has a valid geometryType',
    (id) => {
      const boss = bossCatalog.find((b) => b.id === id) as BossConfig
      expect(VALID_GEOMETRY_TYPES).toContain(boss.geometryType)
    },
  )

  it('all geometry types are distinct', () => {
    const geoTypes = bossCatalog.map((b) => b.geometryType)
    const unique = new Set(geoTypes)
    expect(unique.size).toBe(5)
  })

  it('each boss color is a hex string', () => {
    const hexRe = /^#[0-9a-fA-F]{6}$/
    for (const boss of bossCatalog) {
      expect(boss.color).toMatch(hexRe)
      expect(boss.accentColor).toMatch(hexRe)
    }
  })

  it('each boss scale is a positive number', () => {
    for (const boss of bossCatalog) {
      expect(boss.scale).toBeGreaterThan(0)
    }
  })

  describe('specific boss configs', () => {
    it('the_atom is tier 1 with atom geometry and neon-blue color', () => {
      const boss = bossCatalog.find((b) => b.id === 'the_atom')!
      expect(boss.tier).toBe(1)
      expect(boss.geometryType).toBe('atom')
      expect(boss.color).toBe('#00aaff')
      expect(boss.accentColor).toBe('#00ff88')
      expect(boss.scale).toBe(1.0)
    })

    it('bonding_brothers is tier 2 with dumbbell geometry', () => {
      const boss = bossCatalog.find((b) => b.id === 'bonding_brothers')!
      expect(boss.tier).toBe(2)
      expect(boss.geometryType).toBe('dumbbell')
      expect(boss.color).toBe('#00ff88')
    })

    it('name_lord is tier 3 with grid geometry and neon-gold color', () => {
      const boss = bossCatalog.find((b) => b.id === 'name_lord')!
      expect(boss.tier).toBe(3)
      expect(boss.geometryType).toBe('grid')
      expect(boss.color).toBe('#ffdc00')
    })

    it('the_stereochemist is tier 4 with tetrahedron geometry', () => {
      const boss = bossCatalog.find((b) => b.id === 'the_stereochemist')!
      expect(boss.tier).toBe(4)
      expect(boss.geometryType).toBe('tetrahedron')
      expect(boss.color).toBe('#ff00aa')
    })

    it('the_reactor is tier 5 with flask geometry and orange color', () => {
      const boss = bossCatalog.find((b) => b.id === 'the_reactor')!
      expect(boss.tier).toBe(5)
      expect(boss.geometryType).toBe('flask')
      expect(boss.color).toBe('#ff4400')
    })
  })
})
