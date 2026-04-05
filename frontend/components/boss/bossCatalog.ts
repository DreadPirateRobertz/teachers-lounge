/**
 * Boss catalog — maps the five chemistry bosses to their visual/render configs.
 * Data mirrors services/gaming-service/internal/boss/catalog.go.
 */

import type { BossConfig, BossId } from './types'

/**
 * Ordered array of all five boss configs, tier 1 → 5.
 * Colors use the Weird Science neon palette defined in tailwind.config.
 */
export const bossCatalog: BossConfig[] = [
  {
    id: 'the_atom',
    name: 'THE ATOM',
    tier: 1,
    topic: 'general_chemistry',
    taunt: 'Your electrons are all over the place!',
    color: '#00aaff',       // neon-blue
    accentColor: '#00ff88', // neon-green
    scale: 1.0,
    geometryType: 'atom',
  },
  {
    id: 'bonding_brothers',
    name: 'BONDING BROTHERS',
    tier: 2,
    topic: 'molecular_bonding',
    taunt: 'Weak bonds break — just like that answer!',
    color: '#00ff88',       // neon-green
    accentColor: '#00aaff', // neon-blue
    scale: 1.1,
    geometryType: 'dumbbell',
  },
  {
    id: 'name_lord',
    name: 'NAME LORD',
    tier: 3,
    topic: 'nomenclature',
    taunt: 'You dare mislabel compounds in MY presence?!',
    color: '#ffdc00',       // neon-gold
    accentColor: '#ff00aa', // neon-pink
    scale: 1.0,
    geometryType: 'grid',
  },
  {
    id: 'the_stereochemist',
    name: 'THE STEREOCHEMIST',
    tier: 4,
    topic: 'stereochemistry',
    taunt: 'Mirror, mirror — and your knowledge is shattered!',
    color: '#ff00aa',       // neon-pink
    accentColor: '#ffdc00', // neon-gold
    scale: 1.2,
    geometryType: 'tetrahedron',
  },
  {
    id: 'the_reactor',
    name: 'THE REACTOR',
    tier: 5,
    topic: 'organic_reactions',
    taunt: 'Your reaction mechanism is a catastrophic failure!',
    color: '#ff4400',       // hot orange
    accentColor: '#ff00aa', // neon-pink
    scale: 1.3,
    geometryType: 'flask',
  },
]

/**
 * Look up a single boss config by its ID.
 * Returns `undefined` if the ID is not found (should not happen in practice).
 */
export function getBossConfig(id: BossId): BossConfig | undefined {
  return bossCatalog.find((b) => b.id === id)
}
