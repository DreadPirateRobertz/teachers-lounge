/**
 * @jest-environment jsdom
 */

/**
 * Tests for OnboardingWizard — multi-step first-run setup orchestrator.
 */

import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

// next/navigation is mocked by jest moduleNameMapper in jest.config.ts via
// the __mocks__ directory convention.  We provide a minimal inline mock here.
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), refresh: jest.fn() }),
}))

global.fetch = jest.fn()

import OnboardingWizard from './OnboardingWizard'

const defaultProps = {
  userId: 'user-123',
  displayName: 'Scholar',
  avatarEmoji: '🎓',
}

function mockFetchOk() {
  ;(global.fetch as jest.Mock).mockResolvedValue({
    ok: true,
    status: 204,
    json: async () => ({}),
  })
}

describe('OnboardingWizard', () => {
  beforeEach(() => {
    ;(global.fetch as jest.Mock).mockReset()
  })

  it('renders the welcome step first', () => {
    render(<OnboardingWizard {...defaultProps} />)
    expect(screen.getByText(/Welcome, Scholar!/i)).toBeInTheDocument()
  })

  it('advances to the character step when "Let\'s go" is clicked', () => {
    render(<OnboardingWizard {...defaultProps} />)
    fireEvent.click(screen.getByRole('button', { name: /let's go/i }))
    expect(screen.getByText(/create your character/i)).toBeInTheDocument()
  })

  it('advances to the upload guide after saving character', async () => {
    mockFetchOk()
    render(<OnboardingWizard {...defaultProps} />)

    fireEvent.click(screen.getByRole('button', { name: /let's go/i }))
    // Submit character step with defaults
    fireEvent.click(screen.getByRole('button', { name: /looks good/i }))

    await waitFor(() => {
      expect(screen.getByText(/upload your study materials/i)).toBeInTheDocument()
    })
  })

  it('advances to the ready step after upload guide', async () => {
    mockFetchOk()
    render(<OnboardingWizard {...defaultProps} />)

    // Step 1 → 2
    fireEvent.click(screen.getByRole('button', { name: /let's go/i }))
    // Step 2 → 3
    fireEvent.click(screen.getByRole('button', { name: /looks good/i }))
    await waitFor(() => screen.getByText(/upload your study materials/i))
    // Step 3 → 4
    fireEvent.click(screen.getByRole('button', { name: /got it/i }))

    expect(screen.getByText(/you're all set/i)).toBeInTheDocument()
  })

  it('renders progress dots equal to number of steps', () => {
    render(<OnboardingWizard {...defaultProps} />)
    // 4 steps → 4 progress dots (divs inside progressbar)
    const bar = screen.getByRole('progressbar')
    const dots = bar.querySelectorAll('div')
    expect(dots).toHaveLength(4)
  })

  it('calls PATCH /api/user/profile preferences on character save', async () => {
    mockFetchOk()
    render(<OnboardingWizard {...defaultProps} />)

    fireEvent.click(screen.getByRole('button', { name: /let's go/i }))
    fireEvent.click(screen.getByRole('button', { name: /looks good/i }))

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/api/user/profile/user-123/preferences'),
        expect.objectContaining({ method: 'PATCH' }),
      )
    })
  })
})
