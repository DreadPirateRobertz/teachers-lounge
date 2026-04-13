/**
 * @jest-environment jsdom
 *
 * Tests for the RegisterPage component (`app/(auth)/register/page.tsx`).
 *
 * Covers registration form fields, submit button, navigation link, password
 * validation, form submission (success and error), and loading state.
 */

import React from 'react'
import { render, screen, cleanup, fireEvent, waitFor, act } from '@testing-library/react'
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

const mockRegister = jest.fn()

jest.mock('@/lib/auth', () => ({
  /** Mock register with controllable behavior per test. */
  register: (...args: unknown[]) => mockRegister(...args),
}))

// ── Tests ─────────────────────────────────────────────────────────────────────

afterEach(() => {
  cleanup()
  jest.clearAllMocks()
})

beforeEach(() => {
  mockRegister.mockResolvedValue({
    access_token: 'token',
    user: {
      id: '1',
      email: 'test@test.com',
      display_name: 'ChemWizard',
      subscription_status: 'trial',
    },
  })
})

/** Fill all required fields with valid values and submit the form. */
function fillAndSubmit(overrides: {
  displayName?: string
  email?: string
  password?: string
  confirm?: string
} = {}) {
  const {
    displayName = 'ChemWizard',
    email = 'wizard@test.com',
    password = 'password123',
    confirm = 'password123',
  } = overrides

  fireEvent.change(screen.getByLabelText('Display Name'), { target: { value: displayName } })
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: email } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: password } })
  fireEvent.change(screen.getByLabelText('Confirm Password'), { target: { value: confirm } })
  fireEvent.submit(screen.getByRole('button', { name: /create account/i }).closest('form')!)
}

describe('RegisterPage — static rendering', () => {
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

describe('RegisterPage — validation', () => {
  it('shows error when passwords do not match', async () => {
    render(<RegisterPage />)
    fillAndSubmit({ password: 'password123', confirm: 'different!' })

    await waitFor(() => {
      expect(screen.getByText('Passwords do not match')).toBeInTheDocument()
    })
    expect(mockRegister).not.toHaveBeenCalled()
  })

  it('shows error when password is shorter than 8 characters', async () => {
    render(<RegisterPage />)
    fillAndSubmit({ password: 'short', confirm: 'short' })

    await waitFor(() => {
      expect(screen.getByText('Password must be at least 8 characters')).toBeInTheDocument()
    })
    expect(mockRegister).not.toHaveBeenCalled()
  })
})

describe('RegisterPage — form submission', () => {
  it('calls register with email, password, and display name on valid submit', async () => {
    render(<RegisterPage />)
    fillAndSubmit()

    await waitFor(() => {
      expect(mockRegister).toHaveBeenCalledWith('wizard@test.com', 'password123', 'ChemWizard')
    })
  })

  it('redirects to /subscribe on successful registration', async () => {
    render(<RegisterPage />)
    fillAndSubmit()

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith('/subscribe')
      expect(mockRefresh).toHaveBeenCalled()
    })
  })

  it('displays error message when register throws', async () => {
    mockRegister.mockRejectedValueOnce(new Error('Email already registered'))
    render(<RegisterPage />)
    fillAndSubmit()

    await waitFor(() => {
      expect(screen.getByText('Email already registered')).toBeInTheDocument()
    })
  })

  it('displays fallback error message when non-Error is thrown', async () => {
    mockRegister.mockRejectedValueOnce('network error')
    render(<RegisterPage />)
    fillAndSubmit()

    await waitFor(() => {
      expect(screen.getByText('Registration failed')).toBeInTheDocument()
    })
  })

  it('disables the submit button while loading', async () => {
    let resolveRegister!: () => void
    mockRegister.mockReturnValueOnce(
      new Promise<void>((res) => {
        resolveRegister = res
      }),
    )
    render(<RegisterPage />)
    fillAndSubmit()

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /creating account/i })).toBeDisabled()
    })

    await act(async () => {
      resolveRegister()
    })
  })
})
