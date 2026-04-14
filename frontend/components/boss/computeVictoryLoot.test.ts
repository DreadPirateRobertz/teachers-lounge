/**
 * Unit tests for computeVictoryLoot — the pure loot-derivation helper used by
 * the victory phase of BossBattleClient (tl-dye).
 */
import { computeVictoryLoot } from './BossBattleClient'
import type { BossVisualDef } from './BossCharacterLibrary'

const fakeBoss = {
  id: 'the_atom',
  name: 'The Atom',
  topic: 'general_chemistry',
  primaryColor: '#4af',
  tauntPool: ['electrons everywhere'],
} as unknown as BossVisualDef

describe('computeVictoryLoot', () => {
  it('returns XP, gems, and a boss-specific badge in that order', () => {
    const loot = computeVictoryLoot(fakeBoss, 0)
    expect(loot.map((l) => l.key)).toEqual(['xp', 'gems', 'badge'])
  })

  it('scales XP linearly with turns — 100 base + 10 per turn', () => {
    expect(computeVictoryLoot(fakeBoss, 0)[0].amount).toBe(100)
    expect(computeVictoryLoot(fakeBoss, 5)[0].amount).toBe(150)
    expect(computeVictoryLoot(fakeBoss, 20)[0].amount).toBe(300)
  })

  it('awards a flat gem reward regardless of turns', () => {
    expect(computeVictoryLoot(fakeBoss, 0)[1].amount).toBe(25)
    expect(computeVictoryLoot(fakeBoss, 100)[1].amount).toBe(25)
  })

  it('badge label includes the boss name and has null amount (qualitative)', () => {
    const badge = computeVictoryLoot(fakeBoss, 3)[2]
    expect(badge.label).toBe('Badge: The Atom Slayer')
    expect(badge.amount).toBeNull()
  })

  it('is deterministic for the same inputs', () => {
    const a = computeVictoryLoot(fakeBoss, 7)
    const b = computeVictoryLoot(fakeBoss, 7)
    expect(a).toEqual(b)
  })
})
