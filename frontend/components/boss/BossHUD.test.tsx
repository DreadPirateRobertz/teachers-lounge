/**
 * @jest-environment jsdom
 */

import React from 'react'
import { act, render, screen, fireEvent } from '@testing-library/react'
import BossHUD, { POWER_UPS } from './BossHUD'
import { getBossDef } from './BossCharacterLibrary'

const mockBoss = getBossDef('the_atom')!

const defaultProps = {
  boss: mockBoss,
  bossHP: 75,
  bossMaxHP: 100,
  playerHP: 80,
  playerMaxHP: 100,
  turn: 3,
  gems: 10,
  taunt: null,
  onPowerUpAction: jest.fn(),
  disabled: false,
}

describe('BossHUD', () => {
  beforeEach(() => jest.clearAllMocks())

  it('renders boss name', () => {
    render(<BossHUD {...defaultProps} />)
    expect(screen.getByText('THE ATOM')).toBeInTheDocument()
  })

  it('renders boss tier', () => {
    render(<BossHUD {...defaultProps} />)
    expect(screen.getByText('Tier 1')).toBeInTheDocument()
  })

  it('renders turn number', () => {
    render(<BossHUD {...defaultProps} />)
    expect(screen.getByText('Turn 3')).toBeInTheDocument()
  })

  it('renders boss HP values', () => {
    render(<BossHUD {...defaultProps} />)
    expect(screen.getByText('75 / 100')).toBeInTheDocument()
  })

  it('renders player HP values', () => {
    render(<BossHUD {...defaultProps} bossHP={75} playerHP={80} />)
    expect(screen.getByText('80 / 100')).toBeInTheDocument()
  })

  it('renders gem count', () => {
    render(<BossHUD {...defaultProps} gems={7} />)
    expect(screen.getByText('7')).toBeInTheDocument()
  })

  it('renders all four power-up buttons', () => {
    render(<BossHUD {...defaultProps} />)
    for (const pu of POWER_UPS) {
      expect(screen.getByLabelText(`${pu.label} (${pu.gemCost} gems)`)).toBeInTheDocument()
    }
  })

  it('calls onPowerUpAction with the correct type when a power-up is clicked', () => {
    const handler = jest.fn()
    render(<BossHUD {...defaultProps} onPowerUpAction={handler} />)
    const shieldBtn = screen.getByLabelText('Shield (2 gems)')
    fireEvent.click(shieldBtn)
    expect(handler).toHaveBeenCalledWith('shield')
  })

  it('disables power-up buttons when not enough gems', () => {
    render(<BossHUD {...defaultProps} gems={0} />)
    for (const pu of POWER_UPS) {
      const btn = screen.getByLabelText(`${pu.label} (${pu.gemCost} gems)`)
      expect(btn).toBeDisabled()
    }
  })

  it('disables all buttons when disabled=true', () => {
    render(<BossHUD {...defaultProps} disabled={true} />)
    for (const pu of POWER_UPS) {
      const btn = screen.getByLabelText(`${pu.label} (${pu.gemCost} gems)`)
      expect(btn).toBeDisabled()
    }
  })

  it('does not call onPowerUpAction when disabled', () => {
    const handler = jest.fn()
    render(<BossHUD {...defaultProps} onPowerUpAction={handler} disabled={true} />)
    const healBtn = screen.getByLabelText('Heal (2 gems)')
    fireEvent.click(healBtn)
    expect(handler).not.toHaveBeenCalled()
  })

  it('shows taunt when provided', () => {
    render(<BossHUD {...defaultProps} taunt="Your electrons are all over the place!" />)
    // &ldquo; and &rdquo; render as curly quotes in the DOM
    expect(
      screen.getByText('\u201cYour electrons are all over the place!\u201d'),
    ).toBeInTheDocument()
  })

  it('does not render taunt block when taunt is null', () => {
    render(<BossHUD {...defaultProps} taunt={null} />)
    // No italicized taunt text should be present
    expect(screen.queryByRole('paragraph')).not.toBeInTheDocument()
  })

  it('renders BOSS HP label', () => {
    render(<BossHUD {...defaultProps} />)
    expect(screen.getByText('BOSS HP')).toBeInTheDocument()
  })

  it('renders YOUR HP label', () => {
    render(<BossHUD {...defaultProps} />)
    expect(screen.getByText('YOUR HP')).toBeInTheDocument()
  })

  it('allows clicking affordable power-ups when enabled', () => {
    const handler = jest.fn()
    render(<BossHUD {...defaultProps} gems={5} onPowerUpAction={handler} />)
    const critBtn = screen.getByLabelText('Critical (5 gems)')
    expect(critBtn).not.toBeDisabled()
    fireEvent.click(critBtn)
    expect(handler).toHaveBeenCalledWith('critical')
  })

  describe('combo badge', () => {
    it('omits the combo badge when comboCount is undefined', () => {
      render(<BossHUD {...defaultProps} />)
      expect(screen.queryByTestId('combo-badge')).toBeNull()
    })

    it('omits the combo badge when comboCount is 0 or 1 (below threshold)', () => {
      const { rerender } = render(<BossHUD {...defaultProps} comboCount={0} />)
      expect(screen.queryByTestId('combo-badge')).toBeNull()
      rerender(<BossHUD {...defaultProps} comboCount={1} />)
      expect(screen.queryByTestId('combo-badge')).toBeNull()
    })

    it('shows the combo badge with count when comboCount ≥ 2', () => {
      render(<BossHUD {...defaultProps} comboCount={3} />)
      const badge = screen.getByTestId('combo-badge')
      expect(badge).toBeInTheDocument()
      expect(badge.textContent).toMatch(/3.*COMBO/)
      expect(badge).toHaveAttribute('aria-label', 'Combo streak: 3')
    })

    it('updates the combo badge when count grows mid-battle', () => {
      const { rerender } = render(<BossHUD {...defaultProps} comboCount={2} />)
      expect(screen.getByTestId('combo-badge').textContent).toMatch(/2.*COMBO/)
      rerender(<BossHUD {...defaultProps} comboCount={5} />)
      expect(screen.getByTestId('combo-badge').textContent).toMatch(/5.*COMBO/)
    })
  })

  describe('HP readout tween', () => {
    it('renders the initial HP value immediately on first mount', () => {
      render(<BossHUD {...defaultProps} bossHP={75} bossMaxHP={100} />)
      expect(screen.getByTestId('hp-readout-BOSS HP').textContent).toBe('75 / 100')
    })

    it('animates the HP readout toward the new target when bossHP changes', async () => {
      jest.useFakeTimers()
      let rafCb: FrameRequestCallback | null = null
      const rafSpy = jest.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
        rafCb = cb
        return 1 as unknown as number
      })
      jest.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => {})

      const { rerender } = render(<BossHUD {...defaultProps} bossHP={100} bossMaxHP={100} />)
      expect(screen.getByTestId('hp-readout-BOSS HP').textContent).toBe('100 / 100')

      rerender(<BossHUD {...defaultProps} bossHP={20} bossMaxHP={100} />)
      // Tween completes by HP_TWEEN_MS — drive the rAF callback at t = duration.
      const start = performance.now()
      await act(async () => {
        rafCb?.(start + 10_000)
      })
      expect(screen.getByTestId('hp-readout-BOSS HP').textContent).toBe('20 / 100')

      rafSpy.mockRestore()
      jest.useRealTimers()
    })
  })
})
