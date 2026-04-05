/**
 * @fileoverview Tests for the LeaderboardPanel component.
 *
 * Covers initial loading state, successful data rendering, error handling,
 * period tab switching, and the friends-only toggle.
 *
 * @jest-environment jsdom
 */

import React from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import LeaderboardPanel from './LeaderboardPanel'

// ---------------------------------------------------------------------------
// Shared fixtures
// ---------------------------------------------------------------------------

const GLOBAL_RESPONSE = {
  top_10: [{ user_id: 'u1', xp: 5000, rank: 1, display_name: 'TopStudent' }],
  user_rank: null,
}

const FRIENDS_RESPONSE = {
  friends: [{ user_id: 'f1', xp: 3000, rank: 1, display_name: 'FriendA' }],
  user_rank: null,
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

const mockFetch = jest.fn()

beforeEach(() => {
  global.fetch = mockFetch
})

afterEach(() => {
  mockFetch.mockReset()
  jest.restoreAllMocks()
})

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('LeaderboardPanel', () => {
  /**
   * Renders the component.
   *
   * @returns The RTL render result.
   */
  function setup() {
    return render(<LeaderboardPanel />)
  }

  /**
   * Creates a fetch mock that resolves after the given delay.
   *
   * @param response - The JSON payload to resolve with.
   * @param ok - Whether the response should have ok: true.
   * @param delay - Milliseconds before the promise resolves.
   * @returns A jest mock function configured to return the response.
   */
  function mockSuccess(response: object, ok = true, delay = 0) {
    return {
      ok,
      status: ok ? 200 : 500,
      json: () => new Promise((res) => setTimeout(() => res(response), delay)),
    }
  }

  it('shows loading state initially before fetch resolves', async () => {
    // Provide a slow fetch so loading state is visible during render.
    mockFetch.mockImplementation(
      () =>
        new Promise((resolve) =>
          setTimeout(
            () =>
              resolve({
                ok: true,
                status: 200,
                json: async () => GLOBAL_RESPONSE,
              }),
            200,
          ),
        ),
    )

    setup()

    // The component renders skeleton pulse divs while loading.
    const pulsers = document.querySelectorAll('.animate-pulse')
    expect(pulsers.length).toBeGreaterThan(0)
  })

  it('renders top entries after fetch resolves', async () => {
    mockFetch.mockResolvedValueOnce(mockSuccess(GLOBAL_RESPONSE))

    setup()

    await waitFor(() => {
      expect(screen.getByText('TopStudent')).toBeInTheDocument()
    })

    expect(screen.getByText(/5,000 xp/i)).toBeInTheDocument()
  })

  it('shows error message when fetch fails', async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 500, json: async () => ({}) })

    setup()

    await waitFor(() => {
      expect(screen.getByText(/could not load leaderboard/i)).toBeInTheDocument()
    })

    expect(screen.queryByText('TopStudent')).not.toBeInTheDocument()
  })

  it('switching period tab triggers new fetch with the correct period param', async () => {
    // First call: default all_time mount fetch.
    mockFetch.mockResolvedValueOnce(mockSuccess(GLOBAL_RESPONSE))
    // Second call: after clicking Weekly.
    mockFetch.mockResolvedValueOnce(mockSuccess(GLOBAL_RESPONSE))

    const user = userEvent.setup()
    setup()

    // Wait for initial render to settle.
    await waitFor(() => {
      expect(screen.getByText('TopStudent')).toBeInTheDocument()
    })

    const weeklyTab = screen.getByRole('button', { name: /weekly/i })
    await user.click(weeklyTab)

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(2)
    })

    const secondCallUrl: string = mockFetch.mock.calls[1][0]
    expect(secondCallUrl).toContain('period=weekly')
  })

  it('switching to Monthly tab triggers fetch with period=monthly', async () => {
    mockFetch.mockResolvedValueOnce(mockSuccess(GLOBAL_RESPONSE))
    mockFetch.mockResolvedValueOnce(mockSuccess(GLOBAL_RESPONSE))

    const user = userEvent.setup()
    setup()

    await waitFor(() => screen.getByText('TopStudent'))

    await user.click(screen.getByRole('button', { name: /monthly/i }))

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(2)
    })

    const url: string = mockFetch.mock.calls[1][0]
    expect(url).toContain('period=monthly')
  })

  it('toggling friends-only calls the /api/gaming/leaderboard/friends endpoint', async () => {
    // Mount fetch (all_time global).
    mockFetch.mockResolvedValueOnce(mockSuccess(GLOBAL_RESPONSE))
    // Friends fetch.
    mockFetch.mockResolvedValueOnce(mockSuccess(FRIENDS_RESPONSE))

    const user = userEvent.setup()
    setup()

    await waitFor(() => screen.getByText('TopStudent'))

    // The toggle button starts as "Global".
    const toggleBtn = screen.getByRole('button', { name: /global/i })
    await user.click(toggleBtn)

    await waitFor(() => {
      expect(screen.getByText('FriendA')).toBeInTheDocument()
    })

    // The friends endpoint must have been called.
    const friendsCall = mockFetch.mock.calls.find((args: string[]) =>
      (args[0] as string).includes('/api/gaming/leaderboard/friends'),
    )
    expect(friendsCall).toBeDefined()
  })

  it('toggling friends-only back to global reloads the global feed', async () => {
    // 1. Mount fetch.
    mockFetch.mockResolvedValueOnce(mockSuccess(GLOBAL_RESPONSE))
    // 2. Friends fetch.
    mockFetch.mockResolvedValueOnce(mockSuccess(FRIENDS_RESPONSE))
    // 3. Back to global fetch.
    mockFetch.mockResolvedValueOnce(mockSuccess(GLOBAL_RESPONSE))

    const user = userEvent.setup()
    setup()

    await waitFor(() => screen.getByText('TopStudent'))

    // Turn friends-only on.
    await user.click(screen.getByRole('button', { name: /global/i }))
    await waitFor(() => screen.getByText('FriendA'))

    // Turn friends-only off (button now reads "Friends").
    await user.click(screen.getByRole('button', { name: /friends/i }))
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(3)
    })

    const lastCallUrl: string = mockFetch.mock.calls[2][0]
    expect(lastCallUrl).toContain('/api/gaming/leaderboard')
    expect(lastCallUrl).not.toContain('/friends')
  })
})
