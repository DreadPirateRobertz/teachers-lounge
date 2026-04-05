/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import FlashCard from './FlashCard'
import type { Flashcard } from './FlashCard'

// ── Fixtures ─────────────────────────────────────────────────────────────────

/** Builds a minimal Flashcard fixture with overrideable fields. */
function makeCard(overrides: Partial<Flashcard> = {}): Flashcard {
  return {
    id: 'card-1',
    user_id: 'user-1',
    front: 'What is the capital of France?',
    back: 'Paris',
    source: 'quiz',
    ease_factor: 2.5,
    interval_days: 1,
    repetitions: 0,
    next_review_at: new Date().toISOString(),
    created_at: new Date().toISOString(),
    ...overrides,
  }
}

// ── Tests ────────────────────────────────────────────────────────────────────

describe('FlashCard', () => {
  it('renders front text', () => {
    const card = makeCard()
    render(<FlashCard card={card} onRate={jest.fn()} isRevealed={false} onReveal={jest.fn()} />)
    expect(screen.getByText('What is the capital of France?')).toBeTruthy()
  })

  it('does NOT render back text before reveal', () => {
    const card = makeCard()
    render(<FlashCard card={card} onRate={jest.fn()} isRevealed={false} onReveal={jest.fn()} />)
    // The back text is in the DOM (3-D flip keeps both faces mounted) but the
    // key user-visible indicator is that RatingButtons are absent.
    expect(screen.queryByText('How well did you recall this?')).toBeNull()
  })

  it('renders back text after reveal', () => {
    const card = makeCard()
    render(<FlashCard card={card} onRate={jest.fn()} isRevealed={true} onReveal={jest.fn()} />)
    expect(screen.getByText('Paris')).toBeTruthy()
  })

  it('shows RatingButtons after reveal', () => {
    const card = makeCard()
    render(<FlashCard card={card} onRate={jest.fn()} isRevealed={true} onReveal={jest.fn()} />)
    expect(screen.getByText('How well did you recall this?')).toBeTruthy()
    expect(screen.getByText('Blackout')).toBeTruthy()
  })

  it('does NOT show RatingButtons before reveal', () => {
    const card = makeCard()
    render(<FlashCard card={card} onRate={jest.fn()} isRevealed={false} onReveal={jest.fn()} />)
    expect(screen.queryByText('Blackout')).toBeNull()
  })

  it('calls onReveal when the Show Answer button is pressed', () => {
    const onReveal = jest.fn()
    const card = makeCard()
    render(<FlashCard card={card} onRate={jest.fn()} isRevealed={false} onReveal={onReveal} />)
    fireEvent.click(screen.getByRole('button', { name: /show answer/i }))
    expect(onReveal).toHaveBeenCalledTimes(1)
  })

  it('calls onRate with correct quality when a rating button is pressed', () => {
    const onRate = jest.fn()
    const card = makeCard()
    render(<FlashCard card={card} onRate={onRate} isRevealed={true} onReveal={jest.fn()} />)
    // Click "Perfect" (quality 5)
    fireEvent.click(screen.getByRole('button', { name: /perfect/i }))
    expect(onRate).toHaveBeenCalledWith(5)
  })

  it('applies neon-blue border class when ease_factor >= 2.5', () => {
    const card = makeCard({ ease_factor: 2.5 })
    render(<FlashCard card={card} onRate={jest.fn()} isRevealed={false} onReveal={jest.fn()} />)
    // The card container element has role="button" (the clickable flip area)
    const cardEl = screen.getByRole('button', { name: /click to reveal answer/i })
    expect(cardEl.className).toContain('border-neon-blue')
  })

  it('applies neon-pink border class when ease_factor < 1.8', () => {
    const card = makeCard({ ease_factor: 1.5 })
    render(<FlashCard card={card} onRate={jest.fn()} isRevealed={false} onReveal={jest.fn()} />)
    const cardEl = screen.getByRole('button', { name: /click to reveal answer/i })
    expect(cardEl.className).toContain('border-neon-pink')
  })
})
