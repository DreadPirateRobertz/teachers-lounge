/**
 * @jest-environment jsdom
 *
 * Tests for the LoginPage component (`app/(auth)/login/page.tsx`).
 *
 * Covers form field presence, submit button, navigation link, form submission
 * (success and error), and loading state.
 */

import React from 'react'
import { render, screen, cleanup, fireEvent, waitFor, act } from '@testing-library/react'
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

const mockLogin = jest.fn()

jest.mock('@/lib/auth', () => ({
  /** Mock login with controllable behavior per test. */
  login: (...args: unknown[]) => mockLogin(...args),
}))

// ── Tests ─────────────────────────────────────────────────────────────────────

afterEach(() => {
  cleanup()
  jest.clearAllMocks()
})

beforeEach(() => {
  mockLogin.mockResolvedValue({
    access_token: 'token',
    user: { id: '1', email: 'test@test.com', display_name: 'Test', subscription_status: 'active' },
  })
})

describe('LoginPage — static rendering', () => {
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

describe('LoginPage — form submission', () => {
  it('calls login with email and password on submit', async () => {
    render(<LoginPage />)

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'user@test.com' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'secret123' } })
    fireEvent.submit(screen.getByRole('button', { name: /sign in/i }).closest('form')!)

    await waitFor(() => {
      expect(mockLogin).toHaveBeenCalledWith('user@test.com', 'secret123')
    })
  })

  it('redirects to home on successful login', async () => {
    render(<LoginPage />)

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'user@test.com' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'secret123' } })
    fireEvent.submit(screen.getByRole('button', { name: /sign in/i }).closest('form')!)

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith('/')
      expect(mockRefresh).toHaveBeenCalled()
    })
  })

  it('displays error message when login throws', async () => {
    mockLogin.mockRejectedValueOnce(new Error('Invalid credentials'))
    render(<LoginPage />)

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'bad@test.com' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'wrongpw' } })
    fireEvent.submit(screen.getByRole('button', { name: /sign in/i }).closest('form')!)

    await waitFor(() => {
      expect(screen.getByText('Invalid credentials')).toBeInTheDocument()
    })
  })

  it('displays fallback error message when non-Error is thrown', async () => {
    mockLogin.mockRejectedValueOnce('unexpected failure')
    render(<LoginPage />)

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'bad@test.com' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'wrongpw' } })
    fireEvent.submit(screen.getByRole('button', { name: /sign in/i }).closest('form')!)

    await waitFor(() => {
      expect(screen.getByText('Login failed')).toBeInTheDocument()
    })
  })

  it('disables the submit button while loading', async () => {
    let resolveLogin!: () => void
    mockLogin.mockReturnValueOnce(
      new Promise<void>((res) => {
        resolveLogin = res
      }),
    )
    render(<LoginPage />)

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'user@test.com' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'secret123' } })
    fireEvent.submit(screen.getByRole('button', { name: /sign in/i }).closest('form')!)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /signing in/i })).toBeDisabled()
    })

    await act(async () => {
      resolveLogin()
    })
  })
})
