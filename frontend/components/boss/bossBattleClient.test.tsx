/**
 * @jest-environment jsdom
 *
 * Tests for the ErrorBoundary wrapping of BossScene inside BossBattleClient.
 * Verifies that when BossScene throws a render error the WebGL fallback UI
 * is shown instead of crashing the entire battle page.
 */
import React from 'react'
import { render, screen } from '@testing-library/react'

// ── Module mocks (must precede imports that pull in the mocked modules) ────────

// Silence console.error from React's uncaught-error logging during tests.
const originalConsoleError = console.error
beforeEach(() => {
  console.error = jest.fn()
})
afterEach(() => {
  console.error = originalConsoleError
})

// next/dynamic — collapse to a synchronous passthrough so we can control the
// rendered component without async chunk loading.
jest.mock('next/dynamic', () => ({
  __esModule: true,
  default: (loader: () => Promise<{ default: React.ComponentType }>) => {
    // Return a component that calls the loader's result synchronously.
    // In tests BossScene is replaced by the mock below, so this is safe.
    const Lazy = (props: Record<string, unknown>) => {
      const [Comp, setComp] = React.useState<React.ComponentType | null>(null)
      React.useEffect(() => {
        loader().then((mod) => setComp(() => mod.default))
      }, [])
      return Comp ? <Comp {...props} /> : null
    }
    return Lazy
  },
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
    const { getByRole } = renderBattle()

    // Simulate moving to active phase by clicking "Begin Battle".
    // apiFetch will throw (fetch is not mocked) which is fine — we just need
    // the component to attempt rendering BossScene.
    // Instead, drive phase directly by mocking fetch to return a valid session.
    global.fetch = jest.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          session: {
            session_id: 'sess-1',
            boss_hp: 100,
            boss_max_hp: 100,
            player_hp: 50,
            player_max_hp: 50,
          },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    const startBtn = getByRole('button', { name: /begin battle/i })
    startBtn.click()

    // Wait for the fetch to resolve and state to update.
    await screen.findByTestId('boss-hud')

    // BossScene threw → ErrorBoundary should show the WebGL fallback text.
    expect(screen.getByText(/webgl failed to initialise/i)).toBeInTheDocument()
    expect(screen.getByText(/try refreshing the page/i)).toBeInTheDocument()
  })
})
