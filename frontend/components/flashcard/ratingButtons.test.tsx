/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import RatingButtons from './RatingButtons'

// ── Tests ────────────────────────────────────────────────────────────────────

describe('RatingButtons', () => {
  it('renders all 6 buttons (0-5)', () => {
    render(<RatingButtons onRate={jest.fn()} />)
    const labels = ['Blackout', 'Wrong', 'Hard', 'OK', 'Good', 'Perfect']
    labels.forEach((label) => {
      expect(screen.getByRole('button', { name: new RegExp(label, 'i') })).toBeTruthy()
    })
  })

  it('shows correct label for each button', () => {
    render(<RatingButtons onRate={jest.fn()} />)
    expect(screen.getByText('Blackout')).toBeTruthy()
    expect(screen.getByText('Wrong')).toBeTruthy()
    expect(screen.getByText('Hard')).toBeTruthy()
    expect(screen.getByText('OK')).toBeTruthy()
    expect(screen.getByText('Good')).toBeTruthy()
    expect(screen.getByText('Perfect')).toBeTruthy()
  })

  it('calls onRate with quality 0 when Blackout is clicked', () => {
    const onRate = jest.fn()
    render(<RatingButtons onRate={onRate} />)
    fireEvent.click(screen.getByRole('button', { name: /blackout/i }))
    expect(onRate).toHaveBeenCalledWith(0)
  })

  it('calls onRate with quality 3 when OK is clicked', () => {
    const onRate = jest.fn()
    render(<RatingButtons onRate={onRate} />)
    fireEvent.click(screen.getByRole('button', { name: /ok/i }))
    expect(onRate).toHaveBeenCalledWith(3)
  })

  it('calls onRate with quality 5 when Perfect is clicked', () => {
    const onRate = jest.fn()
    render(<RatingButtons onRate={onRate} />)
    fireEvent.click(screen.getByRole('button', { name: /perfect/i }))
    expect(onRate).toHaveBeenCalledWith(5)
  })

  it('buttons are disabled when disabled prop is true', () => {
    render(<RatingButtons onRate={jest.fn()} disabled={true} />)
    const buttons = screen.getAllByRole('button')
    buttons.forEach((btn) => {
      expect(btn).toBeDisabled()
    })
  })
})
