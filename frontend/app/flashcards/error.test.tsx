/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import FlashcardsError from './error'

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

const testError = new Error('Deck fetch 500') as Error & { digest?: string }

describe('FlashcardsError', () => {
  it('renders the crash heading', () => {
    render(<FlashcardsError error={testError} reset={jest.fn()} />)
    expect(screen.getByText('Flashcard Deck Crashed')).toBeInTheDocument()
  })

  it('renders the "Try again" button', () => {
    render(<FlashcardsError error={testError} reset={jest.fn()} />)
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
  })

  it('calls reset when "Try again" is clicked', () => {
    const reset = jest.fn()
    render(<FlashcardsError error={testError} reset={reset} />)
    fireEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(reset).toHaveBeenCalledTimes(1)
  })

  it('renders back-to-tutor link pointing to /', () => {
    render(<FlashcardsError error={testError} reset={jest.fn()} />)
    expect(screen.getByRole('link', { name: /back to tutor/i })).toHaveAttribute('href', '/')
  })

  it('logs the error to console on mount', () => {
    render(<FlashcardsError error={testError} reset={jest.fn()} />)
    expect(console.error).toHaveBeenCalledWith('[FlashcardsError]', testError)
  })

  it('reassures the user that review history is saved', () => {
    render(<FlashcardsError error={testError} reset={jest.fn()} />)
    expect(screen.getByText(/review history is saved/i)).toBeInTheDocument()
  })
})
