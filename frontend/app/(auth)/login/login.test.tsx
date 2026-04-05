/**
 * @jest-environment jsdom
 *
 * Tests for the LoginPage component (`app/(auth)/login/page.tsx`).
 *
 * Covers form field presence, submit button, and navigation link to the
 * registration page.  Auth calls are mocked so no real network requests are
 * made.
 */

import React from 'react'
import { render, screen, cleanup } from '@testing-library/react'
import LoginPage from './page'

// ── Module mocks ──────────────────────────────────────────────────────────────

jest.mock('next/link', () => {
  /** Mock next/link as a plain anchor element. */
  const MockLink = ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  )
  MockLink.displayName = 'MockLink'
  return MockLink
})

const mockPush = jest.fn()
const mockRefresh = jest.fn()

jest.mock('next/navigation', () => ({
  /** Mock useRouter with controllable push/refresh. */
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
}))

jest.mock('@/lib/auth', () => ({
  /** Mock login to resolve successfully by default. */
  login: jest.fn().mockResolvedValue({
    access_token: 'token',
    user: { id: '1', email: 'test@test.com', display_name: 'Test', subscription_status: 'active' },
  }),
}))

// ── Tests ─────────────────────────────────────────────────────────────────────

afterEach(() => {
  cleanup()
  jest.clearAllMocks()
})

describe('LoginPage', () => {
  it('renders without crashing', () => {
    expect(() => render(<LoginPage />)).not.toThrow()
  })

  it('renders the email input field', () => {
    render(<LoginPage />)
    expect(screen.getByLabelText('Email')).toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toHaveAttribute('type', 'email')
  })

  it('renders the password input field', () => {
    render(<LoginPage />)
    expect(screen.getByLabelText('Password')).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'password')
  })

  it('renders the submit button', () => {
    render(<LoginPage />)
    const submitBtn = screen.getByRole('button', { name: /sign in/i })
    expect(submitBtn).toBeInTheDocument()
    expect(submitBtn).toHaveAttribute('type', 'submit')
  })

  it('renders a link to the register page', () => {
    render(<LoginPage />)
    const registerLink = screen.getByRole('link', { name: /start your free trial/i })
    expect(registerLink).toBeInTheDocument()
    expect(registerLink).toHaveAttribute('href', '/register')
  })

  it('renders the welcome heading', () => {
    render(<LoginPage />)
    expect(screen.getByRole('heading', { name: /welcome back/i })).toBeInTheDocument()
  })
})
