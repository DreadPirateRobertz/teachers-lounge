/**
 * @jest-environment jsdom
 *
 * Tests for AppHeader — static top navigation bar.
 * Verifies logo text, Prof Nova badge, and Analytics nav link.
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import AppHeader from './AppHeader'

jest.mock('next/link', () => {
  const MockLink = ({
    href,
    children,
    ...rest
  }: {
    href: string
    children: React.ReactNode
    [key: string]: unknown
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  )
  MockLink.displayName = 'MockLink'
  return MockLink
})

describe('AppHeader', () => {
  beforeEach(() => {
    render(<AppHeader />)
  })

  it('renders logo text "TV"', () => {
    expect(screen.getByText('TV')).toBeInTheDocument()
  })

  it('renders "Prof Nova" text', () => {
    expect(screen.getByText('Prof Nova')).toBeInTheDocument()
  })

  it('renders Analytics link with href="/analytics"', () => {
    const analyticsLink = screen.getByRole('link', { name: /Analytics/i })
    expect(analyticsLink).toBeInTheDocument()
    expect(analyticsLink).toHaveAttribute('href', '/analytics')
  })
})
