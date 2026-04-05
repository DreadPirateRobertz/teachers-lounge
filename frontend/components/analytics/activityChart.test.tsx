/**
 * @jest-environment jsdom
 *
 * Tests for ActivityChart — GitHub-style heatmap grid component.
 * Verifies square rendering, title attributes, singular/plural message
 * copy, and legend labels.
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import ActivityChart from './ActivityChart'

describe('ActivityChart — empty state', () => {
  it('renders without crashing when days array is empty', () => {
    expect(() => render(<ActivityChart days={[]} />)).not.toThrow()
  })
})

describe('ActivityChart — square rendering', () => {
  const days = [
    { date: '2026-03-01', messages: 5 },
    { date: '2026-03-02', messages: 2 },
    { date: '2026-03-03', messages: 0 },
  ]

  it('renders one square per day entry', () => {
    render(<ActivityChart days={days} />)
    const squares = screen.getAllByTitle(/^2026-03-0/)
    expect(squares).toHaveLength(days.length)
  })

  it('square title contains date and message count', () => {
    render(<ActivityChart days={days} />)
    expect(screen.getByTitle('2026-03-01: 5 messages')).toBeInTheDocument()
    expect(screen.getByTitle('2026-03-02: 2 messages')).toBeInTheDocument()
  })

  it('square title uses singular "message" when count is 1', () => {
    render(<ActivityChart days={[{ date: '2026-03-04', messages: 1 }]} />)
    expect(screen.getByTitle('2026-03-04: 1 message')).toBeInTheDocument()
  })

  it('square title uses plural "messages" when count is 0', () => {
    render(<ActivityChart days={[{ date: '2026-03-05', messages: 0 }]} />)
    expect(screen.getByTitle('2026-03-05: 0 messages')).toBeInTheDocument()
  })

  it('square title uses plural "messages" when count is greater than 1', () => {
    render(<ActivityChart days={[{ date: '2026-03-06', messages: 10 }]} />)
    expect(screen.getByTitle('2026-03-06: 10 messages')).toBeInTheDocument()
  })
})

describe('ActivityChart — legend', () => {
  it('shows "Less" legend label', () => {
    render(<ActivityChart days={[]} />)
    expect(screen.getByText('Less')).toBeInTheDocument()
  })

  it('shows "More" legend label', () => {
    render(<ActivityChart days={[]} />)
    expect(screen.getByText('More')).toBeInTheDocument()
  })
})
