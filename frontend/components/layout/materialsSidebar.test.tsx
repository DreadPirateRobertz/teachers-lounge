/**
 * @jest-environment jsdom
 *
 * Tests for MaterialsSidebar — four-tab panel with Mastery, Rankings,
 * Power-ups, and Materials content. Verifies tab navigation and default
 * active tab.
 */
import React from 'react'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import MaterialsSidebar from './MaterialsSidebar'

jest.mock('./LeaderboardPanel', () => {
  const MockLeaderboardPanel = () => <div data-testid="leaderboard-panel" />
  MockLeaderboardPanel.displayName = 'MockLeaderboardPanel'
  return MockLeaderboardPanel
})

jest.mock('@/components/ErrorBoundary', () => {
  const MockErrorBoundary = ({ children }: { children: React.ReactNode }) => <>{children}</>
  MockErrorBoundary.displayName = 'MockErrorBoundary'
  return MockErrorBoundary
})

// ---------------------------------------------------------------------------
// Mock fetch so the Materials panel polling doesn't make real network calls
// ---------------------------------------------------------------------------

const mockFetch = jest.fn()
beforeEach(() => {
  global.fetch = mockFetch
})
afterEach(() => {
  mockFetch.mockReset()
})

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('MaterialsSidebar — tab labels', () => {
  it('renders all 4 tab labels', () => {
    render(<MaterialsSidebar />)
    expect(screen.getByRole('button', { name: 'Mastery' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rankings' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Power-ups' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Materials' })).toBeInTheDocument()
  })
})

describe('MaterialsSidebar — Mastery tab (default)', () => {
  it('shows mastery topic names by default', () => {
    render(<MaterialsSidebar />)
    expect(screen.getByText('Atomic Structure')).toBeInTheDocument()
    expect(screen.getByText('Chemical Bonding')).toBeInTheDocument()
    expect(screen.getByText('Nomenclature')).toBeInTheDocument()
    expect(screen.getByText('Stereochemistry')).toBeInTheDocument()
    expect(screen.getByText('Organic Reactions')).toBeInTheDocument()
  })

  it('does not show leaderboard panel on initial render', () => {
    render(<MaterialsSidebar />)
    expect(screen.queryByTestId('leaderboard-panel')).not.toBeInTheDocument()
  })
})

describe('MaterialsSidebar — Rankings tab', () => {
  it('shows leaderboard panel after clicking Rankings tab', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar />)
    await user.click(screen.getByRole('button', { name: 'Rankings' }))
    expect(screen.getByTestId('leaderboard-panel')).toBeInTheDocument()
  })

  it('hides mastery topics after clicking Rankings tab', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar />)
    await user.click(screen.getByRole('button', { name: 'Rankings' }))
    expect(screen.queryByText('Atomic Structure')).not.toBeInTheDocument()
  })
})

describe('MaterialsSidebar — Power-ups tab', () => {
  it('shows power-up items after clicking Power-ups tab', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar />)
    await user.click(screen.getByRole('button', { name: 'Power-ups' }))
    expect(screen.getByText('Hint')).toBeInTheDocument()
    expect(screen.getByText('Shield')).toBeInTheDocument()
  })

  it('shows all four power-up names', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar />)
    await user.click(screen.getByRole('button', { name: 'Power-ups' }))
    expect(screen.getByText('Hint')).toBeInTheDocument()
    expect(screen.getByText('Shield')).toBeInTheDocument()
    expect(screen.getByText('2x Damage')).toBeInTheDocument()
    expect(screen.getByText('Time+')).toBeInTheDocument()
  })

  it('hides mastery topics after clicking Power-ups tab', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar />)
    await user.click(screen.getByRole('button', { name: 'Power-ups' }))
    expect(screen.queryByText('Atomic Structure')).not.toBeInTheDocument()
  })
})

describe('MaterialsSidebar — Materials tab', () => {
  it('shows the upload drop zone after clicking Materials tab', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar courseId="11111111-1111-4111-8111-111111111111" />)
    await user.click(screen.getByRole('button', { name: 'Materials' }))
    expect(screen.getByText(/browse/i)).toBeInTheDocument()
    expect(screen.getByText(/drop a file/i)).toBeInTheDocument()
  })

  it('hides mastery topics when Materials tab is active', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar />)
    await user.click(screen.getByRole('button', { name: 'Materials' }))
    expect(screen.queryByText('Atomic Structure')).not.toBeInTheDocument()
  })

  it('shows uploaded material in library after successful upload', async () => {
    const user = userEvent.setup()
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 202,
      json: async () => ({
        job_id: 'job-abc',
        material_id: 'a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5',
        status: 'pending',
      }),
    })

    render(<MaterialsSidebar courseId="11111111-1111-4111-8111-111111111111" />)
    await user.click(screen.getByRole('button', { name: 'Materials' }))

    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    await user.upload(input, new File(['pdf'], 'notes.pdf', { type: 'application/pdf' }))
    await user.click(screen.getByRole('button', { name: /upload material/i }))

    await waitFor(() => expect(screen.getByText('notes.pdf')).toBeInTheDocument())
    expect(screen.getByText('Pending')).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Polling — exercises the MaterialsPanel setInterval path (updates,
// non-ok responses, rejected fetches, and the empty-queue early return).
// ---------------------------------------------------------------------------

describe('MaterialsSidebar — Materials tab polling', () => {
  /**
   * Perform the full upload flow with real timers and wait until the
   * uploaded material is visible in the library in Pending state.
   * The polling setInterval registered during render continues ticking
   * on real time — tests advance it by waiting past POLL_INTERVAL_MS.
   */
  async function uploadPendingMaterial() {
    const user = userEvent.setup()

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 202,
      json: async () => ({
        job_id: 'job-poll',
        material_id: 'b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e',
        status: 'pending',
      }),
    })

    render(<MaterialsSidebar courseId="11111111-1111-4111-8111-111111111111" />)
    await user.click(screen.getByRole('button', { name: 'Materials' }))

    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    await user.upload(input, new File(['pdf'], 'poll.pdf', { type: 'application/pdf' }))
    await user.click(screen.getByRole('button', { name: /upload material/i }))

    await waitFor(() => expect(screen.getByText('Pending')).toBeInTheDocument())
    expect(screen.getByText('poll.pdf')).toBeInTheDocument()
  }

  it('updates a pending material to Ready after a successful poll tick', async () => {
    await uploadPendingMaterial()

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ status: 'complete' }),
    })

    // Wait past the 3s polling interval.
    await waitFor(() => expect(screen.getByText('Ready')).toBeInTheDocument(), {
      timeout: 5_000,
    })
    expect(screen.queryByText('Pending')).not.toBeInTheDocument()
  }, 10_000)

  it('leaves status unchanged when the poll response is not ok', async () => {
    await uploadPendingMaterial()

    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      json: async () => ({ detail: 'boom' }),
    })

    // Wait past one polling tick, then confirm status never changed.
    await new Promise((r) => setTimeout(r, 3_500))
    expect(screen.getByText('Pending')).toBeInTheDocument()
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining('/api/materials/b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e/status'),
    )
  }, 10_000)

  it('leaves status unchanged when the poll fetch rejects', async () => {
    await uploadPendingMaterial()

    mockFetch.mockRejectedValue(new Error('ECONNREFUSED'))

    await new Promise((r) => setTimeout(r, 3_500))
    expect(screen.getByText('Pending')).toBeInTheDocument()
  }, 10_000)

  it('skips the network entirely when there are no non-terminal materials', async () => {
    jest.useFakeTimers()
    render(<MaterialsSidebar courseId="11111111-1111-4111-8111-111111111111" />)

    await act(async () => {
      jest.advanceTimersByTime(3_000 * 3)
    })

    expect(mockFetch).not.toHaveBeenCalled()
    jest.useRealTimers()
  })
})

describe('MaterialsSidebar — Mastery tab click', () => {
  it('returns to mastery tab after switching away', async () => {
    const user = userEvent.setup()
    render(<MaterialsSidebar />)

    await user.click(screen.getByRole('button', { name: 'Rankings' }))
    expect(screen.queryByText('Atomic Structure')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Mastery' }))
    expect(screen.getByText('Atomic Structure')).toBeInTheDocument()
  })
})
