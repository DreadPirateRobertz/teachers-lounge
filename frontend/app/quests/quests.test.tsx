/**
 * @jest-environment jsdom
 *
 * Tests for the QuestsPage component (`app/quests/page.tsx`).
 *
 * Sub-components (AppHeader, XpProgressBar, QuestBoard) are stubbed so the
 * tests focus on the page shell — the back-navigation link, the streak banner,
 * and the quest board container.
 */

import React from 'react'
import { render, screen, cleanup } from '@testing-library/react'
import QuestsPage from './page'

// ── Module mocks ──────────────────────────────────────────────────────────────

jest.mock('next/link', () => {
  /** Mock next/link as a plain anchor element. */
  const MockLink = ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  )
  MockLink.displayName = 'MockLink'
  return MockLink
})

jest.mock('@/components/layout/AppHeader', () => {
  /** Stub AppHeader for quests page tests. */
  const AppHeader = () => <header data-testid="app-header">AppHeader</header>
  AppHeader.displayName = 'AppHeader'
  return AppHeader
})

jest.mock('@/components/layout/XpProgressBar', () => {
  /** Stub XpProgressBar for quests page tests. */
  const XpProgressBar = () => <div data-testid="xp-progress-bar">XpProgressBar</div>
  XpProgressBar.displayName = 'XpProgressBar'
  return XpProgressBar
})

jest.mock('@/components/quests/QuestBoard', () => {
  /** Stub QuestBoard for quests page tests. */
  const QuestBoard = () => <div data-testid="quest-board">QuestBoard</div>
  QuestBoard.displayName = 'QuestBoard'
  return QuestBoard
})

// ── Tests ─────────────────────────────────────────────────────────────────────

afterEach(() => {
  cleanup()
})

describe('QuestsPage', () => {
  it('renders without crashing', () => {
    expect(() => render(<QuestsPage />)).not.toThrow()
  })

  it('renders the AppHeader', () => {
    render(<QuestsPage />)
    expect(screen.getByTestId('app-header')).toBeInTheDocument()
  })

  it('renders the back-to-dashboard link', () => {
    render(<QuestsPage />)
    const backLink = screen.getByRole('link', { name: /back to dashboard/i })
    expect(backLink).toBeInTheDocument()
    expect(backLink).toHaveAttribute('href', '/')
  })

  it('renders the streak banner with streak day count', () => {
    render(<QuestsPage />)
    // The StreakBanner renders "{streak}-Day Streak Active"
    expect(screen.getByText(/7-day streak active/i)).toBeInTheDocument()
  })

  it('renders the XP multiplier value in the streak banner', () => {
    render(<QuestsPage />)
    expect(screen.getByText('×2.0')).toBeInTheDocument()
  })

  it('renders the QuestBoard', () => {
    render(<QuestsPage />)
    expect(screen.getByTestId('quest-board')).toBeInTheDocument()
  })

  it('renders the XpProgressBar', () => {
    render(<QuestsPage />)
    expect(screen.getByTestId('xp-progress-bar')).toBeInTheDocument()
  })

  it('renders the daily quest reset note', () => {
    render(<QuestsPage />)
    expect(screen.getByText(/quests reset every day at midnight utc/i)).toBeInTheDocument()
  })
})
