/**
 * @jest-environment jsdom
 *
 * Tests for QuestionCard — the MCQ question display component used during
 * boss battles. Verifies rendering, option selection, answer reveal, and
 * the difficulty indicator.
 */

import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import QuestionCard, { type BattleQuestion } from './QuestionCard'

const mockQuestion: BattleQuestion = {
  id: 'q-001',
  question: 'What is the atomic number of Carbon?',
  options: [
    { key: 'A', text: '4' },
    { key: 'B', text: '6' },
    { key: 'C', text: '8' },
    { key: 'D', text: '12' },
  ],
  difficulty: 2,
  topic: 'atomic_structure',
  xp_reward: 20,
}

function renderCard(overrides?: Partial<React.ComponentProps<typeof QuestionCard>>) {
  const props = {
    question: mockQuestion,
    chosenKey: null,
    correctKey: null,
    onAnswer: jest.fn(),
    disabled: false,
    ...overrides,
  }
  return { ...render(<QuestionCard {...props} />), onAnswer: props.onAnswer as jest.Mock }
}

describe('QuestionCard', () => {
  it('renders the question text', () => {
    renderCard()
    expect(screen.getByText(/What is the atomic number of Carbon/)).toBeInTheDocument()
  })

  it('renders all four option buttons', () => {
    renderCard()
    for (const opt of mockQuestion.options) {
      expect(screen.getByRole('button', { name: new RegExp(opt.text) })).toBeInTheDocument()
    }
  })

  it('calls onAnswer with the correct key when an option is clicked', () => {
    const { onAnswer } = renderCard()
    fireEvent.click(screen.getByRole('button', { name: /Option B/ }))
    expect(onAnswer).toHaveBeenCalledWith('B')
  })

  it('does not call onAnswer when disabled', () => {
    const { onAnswer } = renderCard({ disabled: true })
    fireEvent.click(screen.getByRole('button', { name: /Option A/ }))
    expect(onAnswer).not.toHaveBeenCalled()
  })

  it('does not call onAnswer a second time after an answer has been chosen', () => {
    const { onAnswer } = renderCard({ chosenKey: 'A' })
    fireEvent.click(screen.getByRole('button', { name: /Option B/ }))
    expect(onAnswer).not.toHaveBeenCalled()
  })

  it('shows the difficulty indicator dots', () => {
    // Difficulty 2 → 2 active dots out of 5.
    const { container } = renderCard()
    // The difficulty row contains 5 span dots.
    const dots = container.querySelectorAll('span.rounded-full.w-1\\.5.h-1\\.5')
    expect(dots).toHaveLength(5)
  })

  it('renders the difficulty label', () => {
    renderCard()
    expect(screen.getByText(/difficulty 2/)).toBeInTheDocument()
  })

  it('disables all option buttons when chosenKey is set (answered state)', () => {
    renderCard({ chosenKey: 'B', correctKey: 'B' })
    for (const opt of mockQuestion.options) {
      const btn = screen.getByRole('button', { name: new RegExp(`Option ${opt.key}`) })
      expect(btn).toBeDisabled()
    }
  })

  it('renders option key prefixes', () => {
    renderCard()
    expect(screen.getByText('A.')).toBeInTheDocument()
    expect(screen.getByText('B.')).toBeInTheDocument()
  })
})
