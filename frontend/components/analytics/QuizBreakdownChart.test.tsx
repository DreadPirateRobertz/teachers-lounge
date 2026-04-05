/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import QuizBreakdownChart from './QuizBreakdownChart'

const topic = (overrides = {}) => ({
  topic: 'Algebra',
  total: 10,
  correct: 8,
  accuracy_pct: 80,
  ...overrides,
})

describe('QuizBreakdownChart — empty state', () => {
  it('shows empty state message when topics array is empty', () => {
    render(<QuizBreakdownChart topics={[]} />)
    expect(screen.getByText(/No quiz data yet/)).toBeInTheDocument()
  })
})

describe('QuizBreakdownChart — with data', () => {
  it('renders topic name', () => {
    render(<QuizBreakdownChart topics={[topic()]} />)
    expect(screen.getByText('Algebra')).toBeInTheDocument()
  })

  it('renders correct/total fraction', () => {
    render(<QuizBreakdownChart topics={[topic({ correct: 7, total: 10 })]} />)
    expect(screen.getByText(/7\/10|7 \/ 10/)).toBeInTheDocument()
  })

  it('renders accuracy percentage', () => {
    render(<QuizBreakdownChart topics={[topic({ accuracy_pct: 80 })]} />)
    expect(screen.getByText(/80%/)).toBeInTheDocument()
  })

  it('renders multiple topics', () => {
    render(<QuizBreakdownChart topics={[
      topic({ topic: 'Algebra' }),
      topic({ topic: 'Geometry', accuracy_pct: 65, correct: 6 }),
    ]} />)
    expect(screen.getByText('Algebra')).toBeInTheDocument()
    expect(screen.getByText('Geometry')).toBeInTheDocument()
  })

  // ── Color thresholds ──────────────────────────────────────────────────

  it('applies green color class for accuracy >= 80%', () => {
    render(<QuizBreakdownChart topics={[topic({ accuracy_pct: 80 })]} />)
    const pctEl = screen.getByText('80%')
    expect(pctEl.className).toContain('neon-green')
  })

  it('applies gold color class for accuracy >= 60% and < 80%', () => {
    render(<QuizBreakdownChart topics={[topic({ accuracy_pct: 65, correct: 6 })]} />)
    const pctEl = screen.getByText('65%')
    expect(pctEl.className).toContain('neon-gold')
  })

  it('applies pink color class for accuracy < 60%', () => {
    render(<QuizBreakdownChart topics={[topic({ accuracy_pct: 45, correct: 4 })]} />)
    const pctEl = screen.getByText('45%')
    expect(pctEl.className).toContain('neon-pink')
  })
})
