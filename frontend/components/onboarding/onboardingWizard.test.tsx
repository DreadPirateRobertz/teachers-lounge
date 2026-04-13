/**
 * @jest-environment jsdom
 */

/**
 * Tests for OnboardingWizard — multi-step first-run setup orchestrator.
 */

import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const mockPush = jest.fn()
const mockRefresh = jest.fn()

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
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

function mockFetchFail(status = 500) {
  ;(global.fetch as jest.Mock).mockResolvedValue({
    ok: false,
    status,
    json: async () => ({ error: 'server error' }),
  })
}

function mockFetchNetworkError() {
  ;(global.fetch as jest.Mock).mockRejectedValue(new Error('ECONNREFUSED'))
}

/** Advance wizard to a given step index by clicking through. */
async function advanceTo(stepIndex: number) {
  if (stepIndex >= 1) {
    fireEvent.click(screen.getByRole('button', { name: /let's go/i }))
  }
  if (stepIndex >= 2) {
    fireEvent.click(screen.getByRole('button', { name: /looks good/i }))
    await waitFor(() => screen.getByText(/upload your study materials/i))
  }
  if (stepIndex >= 3) {
    fireEvent.click(screen.getByRole('button', { name: /got it/i }))
  }
}

describe('OnboardingWizard', () => {
  beforeEach(() => {
    ;(global.fetch as jest.Mock).mockReset()
    mockPush.mockReset()
    mockRefresh.mockReset()
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

  // --- Error state tests ---

  it('stays on character step when save API returns an error', async () => {
    mockFetchFail()
    render(<OnboardingWizard {...defaultProps} />)

    fireEvent.click(screen.getByRole('button', { name: /let's go/i }))
    fireEvent.click(screen.getByRole('button', { name: /looks good/i }))

    await waitFor(() => {
      expect(screen.getByText(/failed to save/i)).toBeInTheDocument()
    })
    // Still on character step
    expect(screen.getByText(/create your character/i)).toBeInTheDocument()
  })

  it('shows completion error when onboarding PATCH fails', async () => {
    // First call (character save) succeeds, second call (onboarding complete) fails
    ;(global.fetch as jest.Mock)
      .mockResolvedValueOnce({ ok: true, status: 204, json: async () => ({}) })
      .mockResolvedValueOnce({ ok: false, status: 500, json: async () => ({}) })

    render(<OnboardingWizard {...defaultProps} />)
    await advanceTo(3)

    fireEvent.click(screen.getByRole('button', { name: /start learning/i }))

    await waitFor(() => {
      expect(screen.getByText(/could not save progress/i)).toBeInTheDocument()
    })
    // Still navigates to home despite error
    expect(mockPush).toHaveBeenCalledWith('/')
  })

  it('shows network error when onboarding PATCH throws', async () => {
    ;(global.fetch as jest.Mock)
      .mockResolvedValueOnce({ ok: true, status: 204, json: async () => ({}) })
      .mockRejectedValueOnce(new Error('ECONNREFUSED'))

    render(<OnboardingWizard {...defaultProps} />)
    await advanceTo(3)

    fireEvent.click(screen.getByRole('button', { name: /start learning/i }))

    await waitFor(() => {
      expect(screen.getByText(/network error/i)).toBeInTheDocument()
    })
    expect(mockPush).toHaveBeenCalledWith('/')
  })

  it('navigates to home and refreshes on successful completion', async () => {
    mockFetchOk()
    render(<OnboardingWizard {...defaultProps} />)
    await advanceTo(3)

    fireEvent.click(screen.getByRole('button', { name: /start learning/i }))

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith('/')
      expect(mockRefresh).toHaveBeenCalled()
    })
  })

  // --- Progress dot state tests ---

  it('marks completed steps with green styling in progress dots', async () => {
    mockFetchOk()
    render(<OnboardingWizard {...defaultProps} />)

    const bar = screen.getByRole('progressbar')
    const dots = bar.querySelectorAll('div')

    // Initially first dot is current (blue), rest are pending
    expect(dots[0].className).toContain('bg-neon-blue')
    expect(dots[1].className).toContain('bg-border-mid')

    // Advance to step 2
    fireEvent.click(screen.getByRole('button', { name: /let's go/i }))
    const updatedDots = bar.querySelectorAll('div')
    expect(updatedDots[0].className).toContain('bg-neon-green')
    expect(updatedDots[1].className).toContain('bg-neon-blue')
  })

  it('shows finishing state on ready step button during completion', async () => {
    let resolveOnboarding!: (value: Response) => void
    ;(global.fetch as jest.Mock)
      .mockResolvedValueOnce({ ok: true, status: 204, json: async () => ({}) })
      .mockImplementationOnce(
        () =>
          new Promise<Response>((r) => {
            resolveOnboarding = r
          }),
      )

    render(<OnboardingWizard {...defaultProps} />)
    await advanceTo(3)

    fireEvent.click(screen.getByRole('button', { name: /start learning/i }))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /launching/i })).toBeDisabled()
    })

    resolveOnboarding({ ok: true, status: 204, json: async () => ({}) } as Response)
  })
})
