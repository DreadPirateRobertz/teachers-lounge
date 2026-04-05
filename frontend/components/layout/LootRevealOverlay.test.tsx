/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, act, fireEvent } from '@testing-library/react'
import LootRevealOverlay, { type LootDrop } from './LootRevealOverlay'

jest.useFakeTimers()

const baseLoot: LootDrop = {
  xp_earned: 200,
  gems_earned: 18,
  achievement: {
    id: 'ach-1',
    achievement_type: 'boss_the_atom',
    badge_name: 'ATOM SMASHER',
    earned_at: '2026-04-04T18:00:00Z',
  },
  cosmetic: {
    key: 'avatar_frame',
    value: 'atomic_ring',
  },
  quote: 'We are all made of star-stuff.',
  new_badge: true,
}

afterEach(() => {
  jest.clearAllTimers()
})

describe('LootRevealOverlay', () => {
  it('renders nothing when loot is null', () => {
    const { container } = render(<LootRevealOverlay loot={null} onClaim={() => {}} />)
    expect(container.firstChild).toBeNull()
  })

  it('shows portal animation on open', () => {
    render(<LootRevealOverlay loot={baseLoot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(250))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('renders VICTORY heading', () => {
    render(<LootRevealOverlay loot={baseLoot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(250))
    expect(screen.getByText('VICTORY!')).toBeInTheDocument()
  })

  it('shows sci-fi quote', () => {
    render(<LootRevealOverlay loot={baseLoot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(250))
    expect(screen.getByText(/star-stuff/)).toBeInTheDocument()
  })

  it('shows XP after 900ms', () => {
    render(<LootRevealOverlay loot={baseLoot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(950))
    expect(screen.getByText('+200 XP')).toBeInTheDocument()
  })

  it('shows gems after 1600ms', () => {
    render(<LootRevealOverlay loot={baseLoot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(1650))
    expect(screen.getByText('+18 Gems')).toBeInTheDocument()
  })

  it('shows badge after 2300ms', () => {
    render(<LootRevealOverlay loot={baseLoot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(2350))
    expect(screen.getByText('ATOM SMASHER')).toBeInTheDocument()
  })

  it('shows NEW BADGE label for first-time earn', () => {
    render(<LootRevealOverlay loot={baseLoot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(2350))
    expect(screen.getByText('NEW BADGE!')).toBeInTheDocument()
  })

  it('shows Badge label for duplicate earn', () => {
    const loot = { ...baseLoot, new_badge: false }
    render(<LootRevealOverlay loot={loot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(2350))
    expect(screen.getByText('Badge')).toBeInTheDocument()
  })

  it('shows cosmetic after 3000ms', () => {
    render(<LootRevealOverlay loot={baseLoot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(3050))
    expect(screen.getByText('Avatar Frame')).toBeInTheDocument()
    expect(screen.getByText('atomic ring')).toBeInTheDocument()
  })

  it('Claim button is disabled before full reveal', () => {
    render(<LootRevealOverlay loot={baseLoot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(250))
    const btn = screen.getByRole('button', { name: /claim/i })
    expect(btn).toBeDisabled()
  })

  it('Claim button is enabled after 3600ms', () => {
    render(<LootRevealOverlay loot={baseLoot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(3700))
    const btn = screen.getByRole('button', { name: /claim/i })
    expect(btn).not.toBeDisabled()
  })

  it('calls onClaim when Claim button is clicked', () => {
    const onClaim = jest.fn()
    render(<LootRevealOverlay loot={baseLoot} onClaim={onClaim} />)
    act(() => jest.advanceTimersByTime(3700))
    // Use fireEvent instead of userEvent to avoid fake-timer conflicts
    // (userEvent v14 uses real timers internally).
    fireEvent.click(screen.getByRole('button', { name: /claim/i }))
    expect(onClaim).toHaveBeenCalledTimes(1)
  })

  it('hides when loot is set to null after showing', () => {
    const { rerender } = render(<LootRevealOverlay loot={baseLoot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(250))
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    rerender(<LootRevealOverlay loot={null} onClaim={() => {}} />)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('renders without achievement or cosmetic', () => {
    const loot: LootDrop = {
      xp_earned: 100,
      gems_earned: 15,
      quote: 'Look up at the stars.',
      new_badge: false,
    }
    render(<LootRevealOverlay loot={loot} onClaim={() => {}} />)
    act(() => jest.advanceTimersByTime(3700))
    expect(screen.getByText('+100 XP')).toBeInTheDocument()
    expect(screen.queryByText('🏆')).toBeNull()
  })
})
