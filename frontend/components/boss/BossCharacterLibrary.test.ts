/**
 * Tests for BossCharacterLibrary — pure data, no Three.js.
 */

import { BOSS_LIBRARY, getBossDef, getBossesInOrder, getRandomTaunt } from './BossCharacterLibrary'

describe('BOSS_LIBRARY', () => {
  it('contains exactly six bosses', () => {
    expect(Object.keys(BOSS_LIBRARY)).toHaveLength(6)
  })

  it('includes all expected boss IDs', () => {
    const ids = Object.keys(BOSS_LIBRARY)
    expect(ids).toContain('the_atom')
    expect(ids).toContain('the_bonder')
    expect(ids).toContain('name_lord')
    expect(ids).toContain('the_stereochemist')
    expect(ids).toContain('the_reactor')
    expect(ids).toContain('final_boss')
  })

  it('has tiers 1 through 6', () => {
    const tiers = Object.values(BOSS_LIBRARY)
      .map((b) => b.tier)
      .sort((a, b) => a - b)
    expect(tiers).toEqual([1, 2, 3, 4, 5, 6])
  })

  describe.each(Object.values(BOSS_LIBRARY))('boss $id', (boss) => {
    it('has required identity fields', () => {
      expect(boss.id).toBeTruthy()
      expect(boss.name).toBeTruthy()
      expect(boss.topic).toBeTruthy()
      expect(boss.tier).toBeGreaterThanOrEqual(1)
      expect(boss.tier).toBeLessThanOrEqual(6)
    })

    it('has a unique procedural geometry', () => {
      // Geometry uniqueness is checked globally below.
      expect(boss.geometry).toBeTruthy()
    })

    it('has valid hex primary color', () => {
      expect(boss.primaryColor).toMatch(/^#[0-9a-fA-F]{6}$/)
    })

    it('has valid hex secondary color', () => {
      expect(boss.secondaryColor).toMatch(/^#[0-9a-fA-F]{6}$/)
    })

    it('has valid hex accent color', () => {
      expect(boss.accentColor).toMatch(/^#[0-9a-fA-F]{6}$/)
    })

    it('has a positive scale', () => {
      expect(boss.scale).toBeGreaterThan(0)
    })

    it('has at least 3 taunts', () => {
      expect(boss.tauntPool.length).toBeGreaterThanOrEqual(3)
    })

    it('has all four animation states', () => {
      expect(boss.animations.idle).toBeDefined()
      expect(boss.animations.attack).toBeDefined()
      expect(boss.animations.damage).toBeDefined()
      expect(boss.animations.death).toBeDefined()
    })

    it('has positive cycleSec for each animation state', () => {
      for (const state of ['idle', 'attack', 'damage', 'death'] as const) {
        expect(boss.animations[state].cycleSec).toBeGreaterThan(0)
      }
    })

    it('has non-negative oscillationAmp for each animation state', () => {
      for (const state of ['idle', 'attack', 'damage', 'death'] as const) {
        expect(boss.animations[state].oscillationAmp).toBeGreaterThanOrEqual(0)
      }
    })

    it('has non-negative particleCount for each animation state', () => {
      for (const state of ['idle', 'attack', 'damage', 'death'] as const) {
        expect(boss.animations[state].particleCount).toBeGreaterThanOrEqual(0)
      }
    })

    it('death animation has more particles than idle', () => {
      expect(boss.animations.death.particleCount).toBeGreaterThan(
        boss.animations.idle.particleCount,
      )
    })

    it('death animation has colorFlash enabled', () => {
      expect(boss.animations.death.colorFlash).toBe(true)
    })
  })

  it('all boss geometries are unique', () => {
    const geometries = Object.values(BOSS_LIBRARY).map((b) => b.geometry)
    const unique = new Set(geometries)
    expect(unique.size).toBe(geometries.length)
  })

  it('tier 6 (final_boss) has the highest scale', () => {
    const scales = Object.values(BOSS_LIBRARY).map((b) => b.scale)
    const finalScale = BOSS_LIBRARY['final_boss'].scale
    const maxScale = Math.max(...scales)
    expect(finalScale).toBe(maxScale)
  })

  it('higher tiers have more death particles (generally)', () => {
    const atom = BOSS_LIBRARY['the_atom'].animations.death.particleCount
    const finalBoss = BOSS_LIBRARY['final_boss'].animations.death.particleCount
    expect(finalBoss).toBeGreaterThan(atom)
  })
})

describe('getBossDef', () => {
  it('returns the correct boss for a valid ID', () => {
    const boss = getBossDef('the_atom')
    expect(boss).toBeDefined()
    expect(boss?.name).toBe('THE ATOM')
    expect(boss?.tier).toBe(1)
  })

  it('returns undefined for an unknown ID', () => {
    expect(getBossDef('not_a_boss')).toBeUndefined()
  })

  it('returns the final boss', () => {
    const boss = getBossDef('final_boss')
    expect(boss?.tier).toBe(6)
    expect(boss?.geometry).toBe('final_boss')
  })
})

describe('getBossesInOrder', () => {
  it('returns all 6 bosses', () => {
    expect(getBossesInOrder()).toHaveLength(6)
  })

  it('returns bosses sorted by tier ascending', () => {
    const ordered = getBossesInOrder()
    for (let i = 0; i < ordered.length - 1; i++) {
      expect(ordered[i].tier).toBeLessThan(ordered[i + 1].tier)
    }
  })

  it('first boss is tier 1, last is tier 6', () => {
    const ordered = getBossesInOrder()
    expect(ordered[0].tier).toBe(1)
    expect(ordered[5].tier).toBe(6)
  })
})

describe('getRandomTaunt', () => {
  it('returns a string for a valid boss', () => {
    const taunt = getRandomTaunt('the_atom')
    expect(typeof taunt).toBe('string')
    expect(taunt.length).toBeGreaterThan(0)
  })

  it('returns a taunt from the pool for the correct boss', () => {
    const boss = getBossDef('the_atom')!
    const taunt = getRandomTaunt('the_atom')
    expect(boss.tauntPool).toContain(taunt)
  })

  it('returns fallback for unknown boss', () => {
    const taunt = getRandomTaunt('no_such_boss')
    expect(taunt).toBe('Wrong!')
  })
})
