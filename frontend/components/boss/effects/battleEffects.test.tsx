/**
 * @jest-environment jsdom
 *
 * Integration tests for BattleEffects composed component and ComboStreakBadge
 * visibility behaviour.
 */

import React from 'react'
import { render, screen } from '@testing-library/react'
import BattleEffects from './BattleEffects'
import ComboStreakBadge from './ComboStreakBadge'

describe('BattleEffects', () => {
  beforeEach(() => {
    jest.spyOn(global, 'requestAnimationFrame').mockImplementation(() => 1)
    jest.spyOn(global, 'cancelAnimationFrame').mockImplementation(() => {})
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('renders without throwing', () => {
    expect(() => {
      render(
        <BattleEffects comboCount={0}>
          <div>battle content</div>
        </BattleEffects>,
      )
    }).not.toThrow()
  })

  it('renders children inside the wrapper', () => {
    render(
      <BattleEffects comboCount={0}>
        <div data-testid="child">hello</div>
      </BattleEffects>,
    )
    expect(screen.getByTestId('child')).toBeDefined()
  })
})

describe('ComboStreakBadge visibility', () => {
  it('badge is not visible when comboCount=0', () => {
    const { container } = render(<ComboStreakBadge comboCount={0} />)
    // ComboStreakBadge returns null at streakLevel 0
    expect(container.firstChild).toBeNull()
  })

  it('badge is visible when comboCount=5', () => {
    render(<ComboStreakBadge comboCount={5} />)
    // At comboCount=5 the badge should display the 2× label
    expect(screen.getByText('2× COMBO')).toBeDefined()
  })

  it('badge is visible when comboCount=3', () => {
    render(<ComboStreakBadge comboCount={3} />)
    expect(screen.getByText('1.5× COMBO')).toBeDefined()
  })
})
