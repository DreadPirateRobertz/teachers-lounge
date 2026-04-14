/**
 * @jest-environment jsdom
 *
 * Unit tests for computeVictoryLoot and VictoryScreen — the pure loot-derivation
 * helper and its rendering wrapper used by the victory phase of BossBattleClient (tl-dye).
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import { computeVictoryLoot, VictoryScreen } from './BossBattleClient'
import type { BossVisualDef } from './BossCharacterLibrary'

// ── Module mocks ───────────────────────────────────────────────────────────────

// LootReveal — render items as a list so tests can assert loot content.
jest.mock('./LootReveal', () => ({
  __esModule: true,
  default: ({ items }: { items: Array<{ key: string; label: string }> }) =>
    React.createElement(
      'div',
      { 'data-testid': 'loot-reveal' },
      items.map((it) => React.createElement('span', { key: it.key }, it.label)),
    ),
}))

// next/link — render as plain <a>.
jest.mock('next/link', () => ({
  __esModule: true,
  default: ({ href, children }: { href: string; children: React.ReactNode }) =>
    React.createElement('a', { href }, children),
}))

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

describe('VictoryScreen', () => {
  it('renders VICTORY heading and the boss name', () => {
    render(<VictoryScreen boss={fakeBoss} turns={5} />)
    expect(screen.getByText('VICTORY!')).toBeInTheDocument()
    expect(screen.getByText('The Atom')).toBeInTheDocument()
  })

  it('renders LootReveal with XP and gem labels', () => {
    render(<VictoryScreen boss={fakeBoss} turns={5} />)
    expect(screen.getByTestId('loot-reveal')).toBeInTheDocument()
    expect(screen.getByText('XP Earned')).toBeInTheDocument()
    expect(screen.getByText('Gems')).toBeInTheDocument()
    expect(screen.getByText('Badge: The Atom Slayer')).toBeInTheDocument()
  })

  it('renders a back link to the tutor home', () => {
    render(<VictoryScreen boss={fakeBoss} turns={0} />)
    expect(screen.getByRole('link', { name: /back to tutor/i })).toHaveAttribute('href', '/')
  })
})
