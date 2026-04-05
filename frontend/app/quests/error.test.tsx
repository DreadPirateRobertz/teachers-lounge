/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import QuestsError from './error'

// Mock next/link so it renders as a plain anchor in tests.
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

const testError = new Error('QuestBoard fetch failed') as Error & { digest?: string }

describe('QuestsError', () => {
  it('renders the crash heading', () => {
    render(<QuestsError error={testError} reset={jest.fn()} />)
    expect(screen.getByText('Quest Board Crashed')).toBeInTheDocument()
  })

  it('renders the "Try again" button', () => {
    render(<QuestsError error={testError} reset={jest.fn()} />)
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
  })

  it('calls reset when "Try again" is clicked', () => {
    const reset = jest.fn()
    render(<QuestsError error={testError} reset={reset} />)
    fireEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(reset).toHaveBeenCalledTimes(1)
  })

  it('renders back-to-tutor link', () => {
    render(<QuestsError error={testError} reset={jest.fn()} />)
    expect(screen.getByRole('link', { name: /back to tutor/i })).toBeInTheDocument()
  })

  it('back-to-tutor link points to /', () => {
    render(<QuestsError error={testError} reset={jest.fn()} />)
    expect(screen.getByRole('link', { name: /back to tutor/i })).toHaveAttribute('href', '/')
  })

  it('logs the error to console on mount', () => {
    render(<QuestsError error={testError} reset={jest.fn()} />)
    expect(console.error).toHaveBeenCalledWith('[QuestsError]', testError)
  })

  it('renders streak reassurance message', () => {
    render(<QuestsError error={testError} reset={jest.fn()} />)
    expect(screen.getByText(/streak is safe/i)).toBeInTheDocument()
  })
})
