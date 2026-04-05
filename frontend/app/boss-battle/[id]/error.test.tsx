/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import BossBattleError from './error'

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

const testError = new Error('WebGL context lost') as Error & { digest?: string }

describe('BossBattleError', () => {
  it('renders the crash heading', () => {
    render(<BossBattleError error={testError} reset={jest.fn()} />)
    expect(screen.getByText('Boss Battle Crashed')).toBeInTheDocument()
  })

  it('renders the "Try again" button', () => {
    render(<BossBattleError error={testError} reset={jest.fn()} />)
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
  })

  it('calls reset when "Try again" is clicked', () => {
    const reset = jest.fn()
    render(<BossBattleError error={testError} reset={reset} />)
    fireEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(reset).toHaveBeenCalledTimes(1)
  })

  it('renders back-to-tutor link', () => {
    render(<BossBattleError error={testError} reset={jest.fn()} />)
    expect(screen.getByRole('link', { name: /back to tutor/i })).toBeInTheDocument()
  })

  it('logs the error to console on mount', () => {
    render(<BossBattleError error={testError} reset={jest.fn()} />)
    expect(console.error).toHaveBeenCalledWith('[BossBattleError]', testError)
  })
})
