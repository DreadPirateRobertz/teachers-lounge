/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, act } from '@testing-library/react'
import LevelUpBanner from './LevelUpBanner'

describe('LevelUpBanner', () => {
  beforeEach(() => {
    jest.useFakeTimers()
  })
  afterEach(() => {
    jest.useRealTimers()
  })

  it('renders the new level number', () => {
    render(<LevelUpBanner newLevel={7} />)
    // level appears twice: once large, once in "You reached Level 7"
    expect(screen.getAllByText('7').length).toBeGreaterThanOrEqual(1)
  })

  it('shows "Level Up!" text', () => {
    render(<LevelUpBanner newLevel={5} />)
    expect(screen.getByText('Level Up!')).toBeInTheDocument()
  })

  it('shows "You reached Level N" copy', () => {
    render(<LevelUpBanner newLevel={10} />)
    expect(screen.getByText(/You reached/)).toBeInTheDocument()
    expect(screen.getByText(/Level 10/)).toBeInTheDocument()
  })

  it('auto-dismisses after 3 seconds', () => {
    render(<LevelUpBanner newLevel={3} />)
    expect(screen.getByText('Level Up!')).toBeInTheDocument()
    act(() => {
      jest.advanceTimersByTime(3000)
    })
    expect(screen.queryByText('Level Up!')).not.toBeInTheDocument()
  })

  it('calls onDismiss callback when timer fires', () => {
    const onDismiss = jest.fn()
    render(<LevelUpBanner newLevel={2} onDismiss={onDismiss} />)
    act(() => {
      jest.advanceTimersByTime(3000)
    })
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('does not call onDismiss before 3 seconds', () => {
    const onDismiss = jest.fn()
    render(<LevelUpBanner newLevel={2} onDismiss={onDismiss} />)
    act(() => {
      jest.advanceTimersByTime(2999)
    })
    expect(onDismiss).not.toHaveBeenCalled()
  })

  it('renders stars decorations', () => {
    render(<LevelUpBanner newLevel={1} />)
    expect(screen.getAllByText('⭐').length).toBe(2)
    expect(screen.getByText('✨')).toBeInTheDocument()
  })
})
