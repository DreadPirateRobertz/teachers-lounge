/**
 * @jest-environment jsdom
 *
 * Tests for ComboStreakBadge — neon combo multiplier badge driven by
 * useComboStreak. Verifies null render at level 0, badge content and
 * aria-label at level > 0, and animate-pulse class toggling.
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import { ComboStreakBadge } from './ComboStreakBadge'
import { useComboStreak } from './useComboStreak'

jest.mock('./useComboStreak', () => ({
  useComboStreak: jest.fn(),
}))

const mockUseComboStreak = useComboStreak as jest.Mock

describe('ComboStreakBadge — no streak', () => {
  beforeEach(() => {
    mockUseComboStreak.mockReturnValue({
      streakLevel: 0,
      streakLabel: '',
      glowColor: '',
      isActive: false,
    })
  })

  afterEach(() => {
    jest.clearAllMocks()
  })

  it('renders nothing when streakLevel is 0', () => {
    const { container } = render(<ComboStreakBadge comboCount={0} />)
    expect(container.firstChild).toBeNull()
  })
})

describe('ComboStreakBadge — active streak', () => {
  afterEach(() => {
    jest.clearAllMocks()
  })

  it('renders badge with streakLabel when streakLevel > 0', () => {
    mockUseComboStreak.mockReturnValue({
      streakLevel: 1,
      streakLabel: '1.5× COMBO',
      glowColor: '#00aaff',
      isActive: true,
    })
    render(<ComboStreakBadge comboCount={3} />)
    expect(screen.getByText('1.5× COMBO')).toBeInTheDocument()
  })

  it('badge has aria-label equal to streakLabel', () => {
    mockUseComboStreak.mockReturnValue({
      streakLevel: 2,
      streakLabel: '2× COMBO',
      glowColor: '#ffdc00',
      isActive: true,
    })
    render(<ComboStreakBadge comboCount={5} />)
    expect(screen.getByRole('generic', { name: '2× COMBO' })).toBeInTheDocument()
  })

  it('badge has animate-pulse class when isActive is true', () => {
    mockUseComboStreak.mockReturnValue({
      streakLevel: 1,
      streakLabel: '1.5× COMBO',
      glowColor: '#00aaff',
      isActive: true,
    })
    render(<ComboStreakBadge comboCount={3} />)
    const badge = screen.getByText('1.5× COMBO')
    expect(badge.className).toContain('animate-pulse')
  })

  it('badge does not have animate-pulse class when isActive is false', () => {
    mockUseComboStreak.mockReturnValue({
      streakLevel: 1,
      streakLabel: '1.5× COMBO',
      glowColor: '#00aaff',
      isActive: false,
    })
    render(<ComboStreakBadge comboCount={3} />)
    const badge = screen.getByText('1.5× COMBO')
    expect(badge.className).not.toContain('animate-pulse')
  })
})
