/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import DeckSummary from './DeckSummary'

// ── Tests ────────────────────────────────────────────────────────────────────

describe('DeckSummary', () => {
  it('shows total card count', () => {
    render(
      <DeckSummary total={42} dueCount={5} onStartReview={jest.fn()} onExportAnki={jest.fn()} />,
    )
    expect(screen.getByText('42')).toBeTruthy()
  })

  it('shows due count', () => {
    render(
      <DeckSummary total={42} dueCount={7} onStartReview={jest.fn()} onExportAnki={jest.fn()} />,
    )
    expect(screen.getByText('7')).toBeTruthy()
  })

  it('"Review Now" button is disabled when dueCount is 0', () => {
    render(
      <DeckSummary total={10} dueCount={0} onStartReview={jest.fn()} onExportAnki={jest.fn()} />,
    )
    const btn = screen.getByRole('button', { name: /no cards due/i })
    expect(btn).toBeDisabled()
  })

  it('"Review Now" button is enabled when dueCount > 0', () => {
    render(
      <DeckSummary total={10} dueCount={3} onStartReview={jest.fn()} onExportAnki={jest.fn()} />,
    )
    const btn = screen.getByRole('button', { name: /review now/i })
    expect(btn).not.toBeDisabled()
  })

  it('calls onStartReview when review button is clicked', () => {
    const onStartReview = jest.fn()
    render(
      <DeckSummary
        total={10}
        dueCount={3}
        onStartReview={onStartReview}
        onExportAnki={jest.fn()}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: /review now/i }))
    expect(onStartReview).toHaveBeenCalledTimes(1)
  })

  it('calls onExportAnki when export button is clicked', () => {
    const onExportAnki = jest.fn()
    render(
      <DeckSummary total={10} dueCount={0} onStartReview={jest.fn()} onExportAnki={onExportAnki} />,
    )
    fireEvent.click(screen.getByRole('button', { name: /export to anki/i }))
    expect(onExportAnki).toHaveBeenCalledTimes(1)
  })
})
