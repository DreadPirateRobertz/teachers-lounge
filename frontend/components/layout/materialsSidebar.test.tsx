/**
 * @jest-environment jsdom
 *
 * Tests for MaterialsSidebar — three-tab panel with Mastery, Rankings,
 * and Power-ups content. Verifies tab navigation and default active tab.
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import MaterialsSidebar from './MaterialsSidebar'

jest.mock('./LeaderboardPanel', () => {
  const MockLeaderboardPanel = () => <div data-testid="leaderboard-panel" />
  MockLeaderboardPanel.displayName = 'MockLeaderboardPanel'
  return MockLeaderboardPanel
})

jest.mock('@/components/ErrorBoundary', () => {
  const MockErrorBoundary = ({ children }: { children: React.ReactNode }) => <>{children}</>
  MockErrorBoundary.displayName = 'MockErrorBoundary'
  return MockErrorBoundary
})

describe('MaterialsSidebar — tab labels', () => {
  it('renders all 3 tab labels', () => {
    render(<MaterialsSidebar />)
    expect(screen.getByRole('button', { name: 'Mastery' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rankings' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Power-ups' })).toBeInTheDocument()
  })
})

describe('MaterialsSidebar — Mastery tab (default)', () => {
  it('shows mastery topic names by default', () => {
    render(<MaterialsSidebar />)
    expect(screen.getByText('Atomic Structure')).toBeInTheDocument()
    expect(screen.getByText('Chemical Bonding')).toBeInTheDocument()
    expect(screen.getByText('Nomenclature')).toBeInTheDocument()
    expect(screen.getByText('Stereochemistry')).toBeInTheDocument()
    expect(screen.getByText('Organic Reactions')).toBeInTheDocument()
  })

  it('does not show leaderboard panel on initial render', () => {
    render(<MaterialsSidebar />)
    expect(screen.queryByTestId('leaderboard-panel')).not.toBeInTheDocument()
  })
})

describe('MaterialsSidebar — Rankings tab', () => {
  it('shows leaderboard panel after clicking Rankings tab', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar />)
    await user.click(screen.getByRole('button', { name: 'Rankings' }))
    expect(screen.getByTestId('leaderboard-panel')).toBeInTheDocument()
  })

  it('hides mastery topics after clicking Rankings tab', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar />)
    await user.click(screen.getByRole('button', { name: 'Rankings' }))
    expect(screen.queryByText('Atomic Structure')).not.toBeInTheDocument()
  })
})

describe('MaterialsSidebar — Power-ups tab', () => {
  it('shows power-up items after clicking Power-ups tab', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar />)
    await user.click(screen.getByRole('button', { name: 'Power-ups' }))
    expect(screen.getByText('Hint')).toBeInTheDocument()
    expect(screen.getByText('Shield')).toBeInTheDocument()
  })

  it('shows all four power-up names', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar />)
    await user.click(screen.getByRole('button', { name: 'Power-ups' }))
    expect(screen.getByText('Hint')).toBeInTheDocument()
    expect(screen.getByText('Shield')).toBeInTheDocument()
    expect(screen.getByText('2x Damage')).toBeInTheDocument()
    expect(screen.getByText('Time+')).toBeInTheDocument()
  })

  it('hides mastery topics after clicking Power-ups tab', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar />)
    await user.click(screen.getByRole('button', { name: 'Power-ups' }))
    expect(screen.queryByText('Atomic Structure')).not.toBeInTheDocument()
  })
})
