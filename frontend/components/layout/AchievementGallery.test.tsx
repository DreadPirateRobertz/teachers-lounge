/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import AchievementGallery from './AchievementGallery'

const USER_ID = 'user-123'

const ATOM_ACH = {
  id: 'ach-1',
  achievement_type: 'boss_the_atom',
  badge_name: 'ATOM SMASHER',
  earned_at: '2026-04-04T18:00:00Z',
}
const BOND_ACH = {
  id: 'ach-2',
  achievement_type: 'boss_bonding_brothers',
  badge_name: 'BOND BREAKER',
  earned_at: '2026-04-05T10:00:00Z',
}

let fetchMock: jest.Mock

beforeEach(() => {
  fetchMock = jest.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ achievements: [ATOM_ACH, BOND_ACH] }),
  } as unknown as Response)
  global.fetch = fetchMock
})

afterEach(() => {
  jest.restoreAllMocks()
})

describe('AchievementGallery — happy path', () => {
  it('fetches achievements for the given userId', async () => {
    render(<AchievementGallery userId={USER_ID} />)
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        `/api/gaming/achievements/${USER_ID}`,
        expect.objectContaining({ cache: 'no-store' }),
      ),
    )
  })

  it('renders badge names after load', async () => {
    render(<AchievementGallery userId={USER_ID} />)
    await waitFor(() => expect(screen.getByText('ATOM SMASHER')).toBeInTheDocument())
    expect(screen.getByText('BOND BREAKER')).toBeInTheDocument()
  })

  it('shows badge count', async () => {
    render(<AchievementGallery userId={USER_ID} />)
    await waitFor(() => expect(screen.getByText('2 badges')).toBeInTheDocument())
  })

  it('shows NEW label for highlightType badge', async () => {
    render(<AchievementGallery userId={USER_ID} highlightType="boss_the_atom" />)
    await waitFor(() => expect(screen.getByText('NEW')).toBeInTheDocument())
  })

  it('does not show NEW label when no highlightType', async () => {
    render(<AchievementGallery userId={USER_ID} />)
    await waitFor(() => expect(screen.getByText('ATOM SMASHER')).toBeInTheDocument())
    expect(screen.queryByText('NEW')).toBeNull()
  })
})

describe('AchievementGallery — empty state', () => {
  it('renders empty message when no achievements', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ achievements: [] }),
    } as unknown as Response)
    render(<AchievementGallery userId={USER_ID} />)
    await waitFor(() => expect(screen.getByText(/defeat bosses/i)).toBeInTheDocument())
  })

  it('shows "0 badges" in header', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ achievements: [] }),
    } as unknown as Response)
    render(<AchievementGallery userId={USER_ID} />)
    await waitFor(() => expect(screen.getByText('0 badges')).toBeInTheDocument())
  })
})

describe('AchievementGallery — error state', () => {
  it('shows error message on fetch failure', async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 500,
      json: async () => ({ error: 'internal error' }),
    } as unknown as Response)
    render(<AchievementGallery userId={USER_ID} />)
    await waitFor(() => expect(screen.getByText(/HTTP 500/)).toBeInTheDocument())
  })
})
