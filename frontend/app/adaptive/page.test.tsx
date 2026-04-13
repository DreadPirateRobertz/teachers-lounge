/**
 * @jest-environment jsdom
 *
 * Tests for the AdaptiveDashboardPage component (`app/adaptive/page.tsx`).
 *
 * Sub-components (AppHeader, XpProgressBar) are stubbed.
 * `global.fetch` is mocked per-test so no real network calls are made.
 */

import React from 'react'
import { render, screen, cleanup, waitFor } from '@testing-library/react'
import AdaptiveDashboardPage from './page'

// ── Mocks ─────────────────────────────────────────────────────────────────────

jest.mock('next/link', () => {
  /** Mock next/link as a plain anchor element. */
  const MockLink = ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  )
  MockLink.displayName = 'MockLink'
  return MockLink
})

jest.mock('@/components/layout/AppHeader', () => {
  /** Stub AppHeader for adaptive page tests. */
  const AppHeader = () => <header data-testid="app-header">AppHeader</header>
  AppHeader.displayName = 'AppHeader'
  return AppHeader
})

jest.mock('@/components/layout/XpProgressBar', () => {
  /** Stub XpProgressBar for adaptive page tests. */
  const XpProgressBar = () => <div data-testid="xp-progress-bar">XpProgressBar</div>
  XpProgressBar.displayName = 'XpProgressBar'
  return XpProgressBar
})

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Build a base64-encoded JWT-like cookie value with the given sub claim. */
function makeCookieToken(userId = 'test-user-id') {
  const payload = btoa(JSON.stringify({ sub: userId }))
  return `header.${payload}.sig`
}

/** Set `tl_token` cookie so getUserIdFromCookie() returns a user ID. */
function setAuthCookie(userId = 'test-user-id') {
  Object.defineProperty(document, 'cookie', {
    writable: true,
    value: `tl_token=${encodeURIComponent(makeCookieToken(userId))}`,
  })
}

/** Clear the auth cookie. */
function clearAuthCookie() {
  Object.defineProperty(document, 'cookie', { writable: true, value: '' })
}

const MASTERY_RESPONSE = {
  concepts: [
    { concept: 'Algebra', correct: 9, total: 10, accuracy_pct: 90.0, mastery_level: 'mastered' },
    { concept: 'Geometry', correct: 4, total: 10, accuracy_pct: 40.0, mastery_level: 'weak' },
  ],
}

const REVIEWS_RESPONSE = {
  reviews: [
    { concept: 'Geometry', due_date: '2026-04-12', days_overdue: 2, priority: 'urgent' },
    { concept: 'Calculus', due_date: '2026-04-14', days_overdue: -2, priority: 'soon' },
  ],
}

const PROFILE_RESPONSE = {
  learning_profile: {
    felder_silverman_dials: {
      active_reflective: -0.6,
      sensing_intuitive: 0.4,
      visual_verbal: -0.2,
      sequential_global: 0.8,
    },
  },
}

/** Mock fetch to return the three successful API responses. */
function mockFetchSuccess() {
  let callCount = 0
  global.fetch = jest.fn().mockImplementation(() => {
    const responses = [MASTERY_RESPONSE, REVIEWS_RESPONSE, PROFILE_RESPONSE]
    const data = responses[callCount % responses.length]
    callCount++
    return Promise.resolve({ ok: true, json: () => Promise.resolve(data) })
  })
}

// ── Tests ─────────────────────────────────────────────────────────────────────

afterEach(() => {
  cleanup()
  clearAuthCookie()
  jest.restoreAllMocks()
})

describe('AdaptiveDashboardPage — unauthenticated', () => {
  it('shows error when no auth cookie is present', async () => {
    clearAuthCookie()
    render(<AdaptiveDashboardPage />)
    await waitFor(() =>
      expect(screen.getByTestId('adaptive-error')).toBeInTheDocument(),
    )
  })
})

describe('AdaptiveDashboardPage — authenticated', () => {
  beforeEach(() => {
    setAuthCookie()
    mockFetchSuccess()
  })

  it('renders without crashing', () => {
    expect(() => render(<AdaptiveDashboardPage />)).not.toThrow()
  })

  it('shows the loading skeleton initially', () => {
    render(<AdaptiveDashboardPage />)
    expect(screen.getByTestId('adaptive-skeleton')).toBeInTheDocument()
  })

  it('renders AppHeader', () => {
    render(<AdaptiveDashboardPage />)
    expect(screen.getByTestId('app-header')).toBeInTheDocument()
  })

  it('renders XpProgressBar', () => {
    render(<AdaptiveDashboardPage />)
    expect(screen.getByTestId('xp-progress-bar')).toBeInTheDocument()
  })

  it('renders the back-to-dashboard link', () => {
    render(<AdaptiveDashboardPage />)
    const link = screen.getByRole('link', { name: /back to dashboard/i })
    expect(link).toHaveAttribute('href', '/')
  })

  it('shows the mastery heatmap after data loads', async () => {
    render(<AdaptiveDashboardPage />)
    await waitFor(() =>
      expect(screen.getByTestId('mastery-heatmap')).toBeInTheDocument(),
    )
  })

  it('shows the upcoming reviews panel after data loads', async () => {
    render(<AdaptiveDashboardPage />)
    await waitFor(() =>
      expect(screen.getByTestId('upcoming-reviews')).toBeInTheDocument(),
    )
  })

  it('shows the weak concept alerts panel after data loads', async () => {
    render(<AdaptiveDashboardPage />)
    await waitFor(() =>
      expect(screen.getByTestId('weak-concept-alerts')).toBeInTheDocument(),
    )
  })

  it('shows the learning style indicator after data loads', async () => {
    render(<AdaptiveDashboardPage />)
    await waitFor(() =>
      expect(screen.getByTestId('learning-style-indicator')).toBeInTheDocument(),
    )
  })

  it('displays weak concept in alert panel', async () => {
    render(<AdaptiveDashboardPage />)
    await waitFor(() => screen.getByTestId('weak-concept-alerts'))
    const elements = screen.getAllByText('Geometry')
    expect(elements.length).toBeGreaterThanOrEqual(1)
  })

  it('displays overdue review concept in upcoming reviews', async () => {
    render(<AdaptiveDashboardPage />)
    await waitFor(() => screen.getByTestId('upcoming-reviews'))
    // "Geometry" also appears in reviews
    const geometryElements = screen.getAllByText('Geometry')
    expect(geometryElements.length).toBeGreaterThanOrEqual(1)
  })

  it('calls the mastery analytics API', async () => {
    render(<AdaptiveDashboardPage />)
    await waitFor(() => screen.getByTestId('adaptive-content'))
    const calls = (global.fetch as jest.Mock).mock.calls
    const masteryCall = calls.find(([url]: [string]) => url.includes('/mastery'))
    expect(masteryCall).toBeDefined()
  })

  it('calls the upcoming-reviews API', async () => {
    render(<AdaptiveDashboardPage />)
    await waitFor(() => screen.getByTestId('adaptive-content'))
    const calls = (global.fetch as jest.Mock).mock.calls
    const reviewCall = calls.find(([url]: [string]) => url.includes('/upcoming-reviews'))
    expect(reviewCall).toBeDefined()
  })

  it('calls the user profile API', async () => {
    render(<AdaptiveDashboardPage />)
    await waitFor(() => screen.getByTestId('adaptive-content'))
    const calls = (global.fetch as jest.Mock).mock.calls
    const profileCall = calls.find(([url]: [string]) => url.includes('/api/user/profile'))
    expect(profileCall).toBeDefined()
  })
})

describe('AdaptiveDashboardPage — fetch error', () => {
  beforeEach(() => {
    setAuthCookie()
    global.fetch = jest.fn().mockRejectedValue(new Error('Network error'))
  })

  it('shows error message when fetch fails', async () => {
    render(<AdaptiveDashboardPage />)
    await waitFor(() =>
      expect(screen.getByTestId('adaptive-error')).toBeInTheDocument(),
    )
  })
})

describe('AdaptiveDashboardPage — no learning style data', () => {
  beforeEach(() => {
    setAuthCookie()
    let callCount = 0
    global.fetch = jest.fn().mockImplementation(() => {
      const responses = [
        MASTERY_RESPONSE,
        REVIEWS_RESPONSE,
        { learning_profile: { felder_silverman_dials: {} } },
      ]
      const data = responses[callCount % responses.length]
      callCount++
      return Promise.resolve({ ok: true, json: () => Promise.resolve(data) })
    })
  })

  it('shows assessment prompt when no dials stored', async () => {
    render(<AdaptiveDashboardPage />)
    await waitFor(() => screen.getByTestId('learning-style-indicator'))
    expect(screen.getByRole('link', { name: /take the assessment/i })).toBeInTheDocument()
  })
})
