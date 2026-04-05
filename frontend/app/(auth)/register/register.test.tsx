/**
 * @jest-environment jsdom
 *
 * Tests for the RegisterPage component (`app/(auth)/register/page.tsx`).
 *
 * Covers registration form fields, submit button, and the navigation link
 * back to the login page.  The register auth call is mocked.
 */

import React from 'react'
import { render, screen, cleanup } from '@testing-library/react'
import RegisterPage from './page'

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
  /** Mock register to resolve successfully by default. */
  register: jest.fn().mockResolvedValue({
    access_token: 'token',
    user: {
      id: '1',
      email: 'test@test.com',
      display_name: 'ChemWizard',
      subscription_status: 'trial',
    },
  }),
}))

// ── Tests ─────────────────────────────────────────────────────────────────────

afterEach(() => {
  cleanup()
  jest.clearAllMocks()
})

describe('RegisterPage', () => {
  it('renders without crashing', () => {
    expect(() => render(<RegisterPage />)).not.toThrow()
  })

  it('renders the display name input field', () => {
    render(<RegisterPage />)
    expect(screen.getByLabelText('Display Name')).toBeInTheDocument()
    expect(screen.getByLabelText('Display Name')).toHaveAttribute('type', 'text')
  })

  it('renders the email input field', () => {
    render(<RegisterPage />)
    expect(screen.getByLabelText('Email')).toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toHaveAttribute('type', 'email')
  })

  it('renders the password input field', () => {
    render(<RegisterPage />)
    expect(screen.getByLabelText('Password')).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'password')
  })

  it('renders the confirm password input field', () => {
    render(<RegisterPage />)
    expect(screen.getByLabelText('Confirm Password')).toBeInTheDocument()
    expect(screen.getByLabelText('Confirm Password')).toHaveAttribute('type', 'password')
  })

  it('renders the submit button', () => {
    render(<RegisterPage />)
    const submitBtn = screen.getByRole('button', { name: /create account/i })
    expect(submitBtn).toBeInTheDocument()
    expect(submitBtn).toHaveAttribute('type', 'submit')
  })

  it('renders a link to the login page', () => {
    render(<RegisterPage />)
    const loginLink = screen.getByRole('link', { name: /sign in/i })
    expect(loginLink).toBeInTheDocument()
    expect(loginLink).toHaveAttribute('href', '/login')
  })

  it('renders the create account heading', () => {
    render(<RegisterPage />)
    expect(screen.getByRole('heading', { name: /create your account/i })).toBeInTheDocument()
  })
})
