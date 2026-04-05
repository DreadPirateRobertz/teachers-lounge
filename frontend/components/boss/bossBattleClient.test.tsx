/**
 * @jest-environment jsdom
 *
 * Tests for the ErrorBoundary wrapping of BossScene inside BossBattleClient.
 * Verifies that when BossScene throws a render error the WebGL fallback UI
 * is shown instead of crashing the entire battle page.
 */
import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'

// ── Module mocks (must precede imports that pull in the mocked modules) ────────

// Silence console.error from React's uncaught-error logging during tests.
const originalConsoleError = console.error
beforeEach(() => {
  console.error = jest.fn()
})
afterEach(() => {
  console.error = originalConsoleError
})

// next/dynamic — return the mocked module synchronously so BossScene renders
// immediately in jsdom without async chunk loading or useEffect delays.
jest.mock('next/dynamic', () => ({
  __esModule: true,
  default: () => jest.requireMock('./BossScene').default,
}))

// BossScene mock — default export throws so the ErrorBoundary triggers.
jest.mock('./BossScene', () => ({
  __esModule: true,
  default: function BossSceneMock() {
    throw new Error('WebGL context lost')
  },
}))

// next/link — render as plain <a> to avoid router context requirements.
jest.mock('next/link', () => ({
  __esModule: true,
  default: ({
    href,
    children,
    className,
  }: {
    href: string
    children: React.ReactNode
    className?: string
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}))

// BossHUD and BossCharacterLibrary — stub out to keep the test focused.
jest.mock('./BossHUD', () => ({
  __esModule: true,
  default: () => <div data-testid="boss-hud" />,
}))

jest.mock('./BossCharacterLibrary', () => ({
  __esModule: true,
  getRandomTaunt: () => 'mock taunt',
  BOSS_LIBRARY: {},
}))

// ── Test helpers ──────────────────────────────────────────────────────────────

import BossBattleClient from './BossBattleClient'
import { type BossVisualDef } from './BossCharacterLibrary'

/** Minimal boss definition sufficient to render BossBattleClient. */
const mockBoss: BossVisualDef = {
  id: 'test_boss',
  name: 'Test Boss',
  topic: 'math',
  primaryColor: '#ff0099',
  tauntPool: ['You shall not pass!'],
  modelPath: '/models/test.glb',
  scale: 1,
  ambientColor: '#000',
  pointColor: '#fff',
}

function renderBattle() {
  return render(<BossBattleClient boss={mockBoss} userId="user-1" initialGems={10} />)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('BossBattleClient — ErrorBoundary around BossScene', () => {
  it('renders the start screen initially (BossScene not mounted yet)', () => {
    renderBattle()
    // BossScene is only rendered when phase === 'active', so the start screen
    // should be visible without triggering the WebGL mock throw.
    expect(screen.getByRole('button', { name: /begin battle/i })).toBeInTheDocument()
  })

  it('shows WebGL fallback when BossScene throws during active phase', async () => {
    // Mock fetch so startBattle() transitions phase → 'active', which mounts
    // BossScene. The synchronous dynamic mock makes BossScene render immediately,
    // throw, and let the ErrorBoundary show its fallback.
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: jest.fn().mockResolvedValue({
        session: {
          session_id: 'sess-1',
          boss_hp: 100,
          boss_max_hp: 100,
          player_hp: 50,
          player_max_hp: 50,
        },
      }),
    })

    const { getByRole } = renderBattle()
    getByRole('button', { name: /begin battle/i }).click()

    // BossScene threw → ErrorBoundary should show the WebGL fallback text.
    await waitFor(() => {
      expect(screen.getByText(/webgl failed to initialise/i)).toBeInTheDocument()
    })
    expect(screen.getByText(/try refreshing the page/i)).toBeInTheDocument()
  })
})
