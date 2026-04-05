/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import FlashcardsReviewError from './error'

jest.mock('next/link', () => ({
  __esModule: true,
  default: ({
    href,
    children,
    className,
  }: {
    href: string
    children: React.ReactNode
    className?: string
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}))

const originalConsoleError = console.error
beforeEach(() => {
  console.error = jest.fn()
})
afterEach(() => {
  console.error = originalConsoleError
})

const testError = new Error('FlashCard flip exploded') as Error & { digest?: string }

describe('FlashcardsReviewError', () => {
  it('renders the crash heading', () => {
    render(<FlashcardsReviewError error={testError} reset={jest.fn()} />)
    expect(screen.getByText('Review Session Crashed')).toBeInTheDocument()
  })

  it('renders the "Try again" button', () => {
    render(<FlashcardsReviewError error={testError} reset={jest.fn()} />)
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
  })

  it('calls reset when "Try again" is clicked', () => {
    const reset = jest.fn()
    render(<FlashcardsReviewError error={testError} reset={reset} />)
    fireEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(reset).toHaveBeenCalledTimes(1)
  })

  it('renders back-to-deck link pointing to /flashcards', () => {
    render(<FlashcardsReviewError error={testError} reset={jest.fn()} />)
    expect(screen.getByRole('link', { name: /back to deck/i })).toHaveAttribute(
      'href',
      '/flashcards',
    )
  })

  it('logs the error to console on mount', () => {
    render(<FlashcardsReviewError error={testError} reset={jest.fn()} />)
    expect(console.error).toHaveBeenCalledWith('[FlashcardsReviewError]', testError)
  })

  it('reassures the user that submitted ratings are saved', () => {
    render(<FlashcardsReviewError error={testError} reset={jest.fn()} />)
    expect(screen.getByText(/ratings you already submitted are saved/i)).toBeInTheDocument()
  })
})
