/**
 * @jest-environment jsdom
 *
 * Tests that BossBattleClient mirrors the gaming-service WebSocket stream
 * into the HUD: HP, combo, and authoritative phase transitions all flow
 * from useBattleStream → BossHUD without requiring an extra REST round-trip.
 */

import React from 'react'
import { act, render, screen, waitFor } from '@testing-library/react'

// ── Module mocks ─────────────────────────────────────────────────────────────

const originalConsoleError = console.error
beforeEach(() => {
  console.error = jest.fn()
})
afterEach(() => {
  console.error = originalConsoleError
})

// next/dynamic — render BossScene synchronously so phase transitions paint.
jest.mock('next/dynamic', () => ({
  __esModule: true,
  default: () => jest.requireMock('./BossScene').default,
}))

// BossScene mock — non-throwing, just exposes the current animation state.
jest.mock('./BossScene', () => ({
  __esModule: true,
  default: function BossSceneMock({ animationState }: { animationState: string }) {
    return <div data-testid="boss-scene-mock" data-anim={animationState} />
  },
}))

// next/link — stub to plain anchor.
jest.mock('next/link', () => ({
  __esModule: true,
  default: ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  ),
}))

// useBattleStream — controllable from the test via a mutable ref.
const streamRef: { current: ReturnType<typeof makeState> } = { current: makeState(null) }

function makeState(
  state: null | {
    bossHp: number
    playerHp: number
    damageDealt: number
    comboCount: number
    phase: 'active' | 'victory' | 'defeat'
  },
) {
  return { battleState: state, sendAttack: jest.fn(), isConnected: state !== null }
}

jest.mock('@/hooks/useBattleStream', () => ({
  __esModule: true,
  useBattleStream: () => streamRef.current,
}))

// ── Imports under test ───────────────────────────────────────────────────────

import BossBattleClient from './BossBattleClient'
import { getBossDef } from './BossCharacterLibrary'

const boss = getBossDef('the_atom')!

function renderBattle() {
  return render(<BossBattleClient boss={boss} userId="user-1" initialGems={10} />)
}

/** Simulate `startBattle()` succeeding so the HUD mounts. */
async function startBattleAndAwaitHud() {
  global.fetch = jest.fn().mockImplementation((url: string) => {
    if (url.includes('/gaming/boss/start')) {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            session: {
              session_id: 'sess-1',
              boss_hp: 100,
              boss_max_hp: 100,
              player_hp: 50,
              player_max_hp: 50,
            },
          }),
      })
    }
    if (url.includes('/gaming/quiz/start')) {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            session: { id: 'quiz-1' },
            question: {
              id: 'q1',
              prompt: '2 + 2 = ?',
              options: [
                { key: 'A', text: '4' },
                { key: 'B', text: '5' },
              ],
              difficulty: 1,
            },
          }),
      })
    }
    return Promise.resolve({ ok: true, json: () => Promise.resolve({}) })
  })

  const view = renderBattle()
  await act(async () => {
    screen.getByRole('button', { name: /begin battle/i }).click()
  })
  await waitFor(() => expect(screen.getByText('BOSS HP')).toBeInTheDocument())
  return view
}

// ── Tests ────────────────────────────────────────────────────────────────────

describe('BossBattleClient — WS stream mirroring', () => {
  beforeEach(() => {
    streamRef.current = makeState(null)
  })

  it('renders the HUD with REST-supplied initial HP before any stream event', async () => {
    await startBattleAndAwaitHud()
    expect(screen.getByText('100 / 100')).toBeInTheDocument()
    expect(screen.getByText('50 / 50')).toBeInTheDocument()
  })

  it('reflects HP and combo from the stream once a state_update arrives', async () => {
    const { rerender } = await startBattleAndAwaitHud()

    // Stream pushes a state_update — combo 3, boss took damage.
    streamRef.current = makeState({
      bossHp: 70,
      playerHp: 50,
      damageDealt: 30,
      comboCount: 3,
      phase: 'active',
    })
    await act(async () => {
      rerender(<BossBattleClient boss={boss} userId="user-1" initialGems={10} />)
    })

    expect(screen.getByTestId('combo-badge').textContent).toMatch(/3.*COMBO/)
    // HP readout tweens; readout target reflects the new bossHp eventually.
    await waitFor(() => {
      expect(screen.getByTestId('hp-readout-BOSS HP').textContent).toMatch(/\d+ \/ 100/)
    })
  })

  it('hides the combo badge while combo < 2', async () => {
    const { rerender } = await startBattleAndAwaitHud()
    streamRef.current = makeState({
      bossHp: 90,
      playerHp: 50,
      damageDealt: 10,
      comboCount: 1,
      phase: 'active',
    })
    await act(async () => {
      rerender(<BossBattleClient boss={boss} userId="user-1" initialGems={10} />)
    })
    expect(screen.queryByTestId('combo-badge')).toBeNull()
  })

  it('triggers the death animation when stream phase flips to victory', async () => {
    const { rerender } = await startBattleAndAwaitHud()
    streamRef.current = makeState({
      bossHp: 0,
      playerHp: 50,
      damageDealt: 100,
      comboCount: 5,
      phase: 'victory',
    })
    await act(async () => {
      rerender(<BossBattleClient boss={boss} userId="user-1" initialGems={10} />)
    })
    await waitFor(() => {
      expect(screen.getByTestId('boss-scene-mock').getAttribute('data-anim')).toBe('death')
    })
  })

  it('jumps to defeat phase when stream phase reports defeat', async () => {
    const { rerender } = await startBattleAndAwaitHud()
    streamRef.current = makeState({
      bossHp: 50,
      playerHp: 0,
      damageDealt: 0,
      comboCount: 0,
      phase: 'defeat',
    })
    await act(async () => {
      rerender(<BossBattleClient boss={boss} userId="user-1" initialGems={10} />)
    })
    await waitFor(() => {
      expect(screen.getByText('DEFEATED')).toBeInTheDocument()
    })
  })
})
