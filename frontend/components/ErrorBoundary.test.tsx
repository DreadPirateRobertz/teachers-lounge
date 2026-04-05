/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import ErrorBoundary from './ErrorBoundary'

/** Test helper that throws on demand. */
function Bomb({ shouldThrow }: { shouldThrow: boolean }) {
  if (shouldThrow) throw new Error('Test explosion')
  return <div>Safe content</div>
}

// Silence expected React error output so test output stays clean.
const originalConsoleError = console.error
beforeEach(() => {
  console.error = jest.fn()
})
afterEach(() => {
  console.error = originalConsoleError
})

describe('ErrorBoundary', () => {
  it('renders children when no error occurs', () => {
    render(
      <ErrorBoundary>
        <div>Hello world</div>
      </ErrorBoundary>,
    )
    expect(screen.getByText('Hello world')).toBeInTheDocument()
  })

  it('shows default fallback when a child throws', () => {
    render(
      <ErrorBoundary>
        <Bomb shouldThrow />
      </ErrorBoundary>,
    )
    expect(screen.getByText(/failed to load/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
  })

  it('includes componentName in default fallback', () => {
    render(
      <ErrorBoundary componentName="Leaderboard">
        <Bomb shouldThrow />
      </ErrorBoundary>,
    )
    expect(screen.getByText(/Leaderboard/)).toBeInTheDocument()
  })

  it('uses "This section" when componentName is omitted', () => {
    render(
      <ErrorBoundary>
        <Bomb shouldThrow />
      </ErrorBoundary>,
    )
    expect(screen.getByText(/This section/)).toBeInTheDocument()
  })

  it('renders custom fallback when provided', () => {
    render(
      <ErrorBoundary fallback={<div>Custom fallback</div>}>
        <Bomb shouldThrow />
      </ErrorBoundary>,
    )
    expect(screen.getByText('Custom fallback')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /try again/i })).not.toBeInTheDocument()
  })

  it('recovers after clicking "Try again" when child no longer throws', () => {
    const { rerender } = render(
      <ErrorBoundary>
        <Bomb shouldThrow />
      </ErrorBoundary>,
    )
    // Update child to not throw before resetting, so the re-render after reset succeeds.
    rerender(
      <ErrorBoundary>
        <Bomb shouldThrow={false} />
      </ErrorBoundary>,
    )
    fireEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(screen.getByText('Safe content')).toBeInTheDocument()
  })

  it('logs the error to console on crash', () => {
    render(
      <ErrorBoundary componentName="Quiz">
        <Bomb shouldThrow />
      </ErrorBoundary>,
    )
    expect(console.error).toHaveBeenCalled()
  })

  it('does not show fallback when child stops throwing after reset', () => {
    const { rerender } = render(
      <ErrorBoundary>
        <Bomb shouldThrow />
      </ErrorBoundary>,
    )
    rerender(
      <ErrorBoundary>
        <Bomb shouldThrow={false} />
      </ErrorBoundary>,
    )
    fireEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(screen.queryByText(/failed to load/i)).not.toBeInTheDocument()
  })
})
