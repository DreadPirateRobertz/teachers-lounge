/**
 * @jest-environment jsdom
 *
 * Unit tests for LearningStyleBadge — dominant-axis selection and
 * accessible badge rendering for the chat header (tl-h9e).
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import LearningStyleBadge, { pickStyleLabel, type FelderDials } from './LearningStyleBadge'

const ZERO: FelderDials = {
  active_reflective: 0,
  sensing_intuitive: 0,
  visual_verbal: 0,
  sequential_global: 0,
}

describe('pickStyleLabel', () => {
  it('returns Balanced when dials is null', () => {
    expect(pickStyleLabel(null).label).toBe('Balanced')
  })

  it('returns Balanced when no axis crosses the dominance threshold', () => {
    expect(pickStyleLabel(ZERO).label).toBe('Balanced')
    expect(pickStyleLabel({ ...ZERO, visual_verbal: 0.1 }).label).toBe('Balanced')
  })

  it('picks Visual when visual_verbal is strongly negative', () => {
    expect(pickStyleLabel({ ...ZERO, visual_verbal: -0.7 }).label).toBe('Visual')
  })

  it('picks Auditory when visual_verbal is strongly positive', () => {
    expect(pickStyleLabel({ ...ZERO, visual_verbal: 0.9 }).label).toBe('Auditory')
  })

  it('picks Active when active_reflective is strongly negative', () => {
    expect(pickStyleLabel({ ...ZERO, active_reflective: -0.5 }).label).toBe('Active')
  })

  it('picks Reflective when active_reflective is strongly positive', () => {
    expect(pickStyleLabel({ ...ZERO, active_reflective: 0.6 }).label).toBe('Reflective')
  })

  it('picks Sensing/Intuitive on the sensing_intuitive axis', () => {
    expect(pickStyleLabel({ ...ZERO, sensing_intuitive: -0.4 }).label).toBe('Sensing')
    expect(pickStyleLabel({ ...ZERO, sensing_intuitive: 0.4 }).label).toBe('Intuitive')
  })

  it('picks Sequential/Global on the sequential_global axis', () => {
    expect(pickStyleLabel({ ...ZERO, sequential_global: -0.3 }).label).toBe('Sequential')
    expect(pickStyleLabel({ ...ZERO, sequential_global: 0.3 }).label).toBe('Global')
  })

  it('picks the axis with the largest absolute magnitude when several cross threshold', () => {
    const dials: FelderDials = {
      active_reflective: -0.3,
      sensing_intuitive: 0.4,
      visual_verbal: -0.9,
      sequential_global: 0.5,
    }
    expect(pickStyleLabel(dials).label).toBe('Visual')
  })
})

describe('LearningStyleBadge', () => {
  it('renders with role-describing aria-label', () => {
    render(<LearningStyleBadge dials={null} />)
    const badge = screen.getByTestId('learning-style-badge')
    expect(badge).toHaveAttribute('aria-label', 'Learning style: Balanced')
    expect(badge).toHaveTextContent('Balanced')
  })

  it('updates label when dials change to favor Visual', () => {
    const { rerender } = render(<LearningStyleBadge dials={null} />)
    expect(screen.getByTestId('learning-style-badge')).toHaveTextContent('Balanced')

    rerender(<LearningStyleBadge dials={{ ...ZERO, visual_verbal: -0.7 }} />)
    const badge = screen.getByTestId('learning-style-badge')
    expect(badge).toHaveAttribute('aria-label', 'Learning style: Visual')
    expect(badge).toHaveTextContent('Visual')
  })

  it('renders an icon alongside the label', () => {
    render(<LearningStyleBadge dials={{ ...ZERO, visual_verbal: 0.9 }} />)
    const badge = screen.getByTestId('learning-style-badge')
    // Auditory icon
    expect(badge.textContent).toContain('🎧')
  })
})
