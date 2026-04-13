/**
 * @jest-environment jsdom
 *
 * Tests for BattleResolve — the damage-number overlay shown after each
 * battle round. Verifies correct/wrong banners, damage display, and
 * the optional explanation text.
 */

import React from 'react'
import { render, screen } from '@testing-library/react'
import BattleResolve from './BattleResolve'

describe('BattleResolve', () => {
  it('shows CORRECT banner for a correct answer', () => {
    render(
      <BattleResolve
        playerDamage={40}
        bossDamage={10}
        correct={true}
        explanation="Carbon has 6 protons."
      />,
    )
    expect(screen.getByText(/✓ CORRECT/)).toBeInTheDocument()
  })

  it('shows WRONG banner for an incorrect answer', () => {
    render(<BattleResolve playerDamage={0} bossDamage={20} correct={false} explanation="" />)
    expect(screen.getByText(/✗ WRONG/)).toBeInTheDocument()
  })

  it('displays player damage dealt when greater than 0', () => {
    render(<BattleResolve playerDamage={40} bossDamage={10} correct={true} explanation="" />)
    expect(screen.getByText('-40')).toBeInTheDocument()
    expect(screen.getByText('to boss')).toBeInTheDocument()
  })

  it('displays boss damage dealt when greater than 0', () => {
    render(<BattleResolve playerDamage={0} bossDamage={20} correct={false} explanation="" />)
    expect(screen.getByText('-20')).toBeInTheDocument()
    expect(screen.getByText('to you')).toBeInTheDocument()
  })

  it('hides player damage row when playerDamage is 0', () => {
    render(<BattleResolve playerDamage={0} bossDamage={15} correct={false} explanation="" />)
    expect(screen.queryByText('to boss')).not.toBeInTheDocument()
  })

  it('shows "no damage" when both values are 0', () => {
    render(<BattleResolve playerDamage={0} bossDamage={0} correct={false} explanation="" />)
    expect(screen.getByText(/No damage this round/)).toBeInTheDocument()
  })

  it('renders explanation text when provided', () => {
    render(
      <BattleResolve
        playerDamage={40}
        bossDamage={0}
        correct={true}
        explanation="Carbon is in group 14 with 6 electrons."
      />,
    )
    expect(screen.getByText(/Carbon is in group 14/)).toBeInTheDocument()
  })

  it('does not render explanation block when explanation is empty', () => {
    const { container } = render(
      <BattleResolve playerDamage={0} bossDamage={20} correct={false} explanation="" />,
    )
    // Explanation paragraph uses a <p> tag — should not be present.
    const paras = container.querySelectorAll('p')
    for (const p of paras) {
      expect(p.textContent).not.toMatch(/Carbon/)
    }
  })

  it('has aria-live="polite" for screen reader announcement', () => {
    const { container } = render(
      <BattleResolve playerDamage={10} bossDamage={5} correct={true} explanation="" />,
    )
    const root = container.firstElementChild
    expect(root?.getAttribute('aria-live')).toBe('polite')
  })
})
