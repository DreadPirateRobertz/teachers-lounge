/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import QuestBoard from './QuestBoard'

const QUEST = {
  id: 'questions_answered',
  title: 'Answer 5 Questions',
  description: 'Ask Prof Nova 5 questions today.',
  progress: 2,
  target: 5,
  completed: false,
  xp_reward: 50,
  gems_reward: 10,
}

let fetchMock: jest.Mock

beforeEach(() => {
  fetchMock = jest.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ quests: [QUEST] }),
  } as unknown as Response)
  global.fetch = fetchMock
})

afterEach(() => {
  jest.restoreAllMocks()
})

describe('QuestBoard — happy path', () => {
  it('fetches quests from /api/gaming/quests on mount', async () => {
    render(<QuestBoard />)
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(
      '/api/gaming/quests',
      expect.objectContaining({ cache: 'no-store' }),
    ))
  })

  it('renders quest title after load', async () => {
    render(<QuestBoard />)
    await waitFor(() => expect(screen.getByText('Answer 5 Questions')).toBeInTheDocument())
  })

  it('renders quest description', async () => {
    render(<QuestBoard />)
    await waitFor(() => expect(screen.getByText('Ask Prof Nova 5 questions today.')).toBeInTheDocument())
  })

  it('renders progress fraction', async () => {
    render(<QuestBoard />)
    await waitFor(() => expect(screen.getByText(/2 \/ 5|2\/5/)).toBeInTheDocument())
  })

  it('renders XP reward', async () => {
    render(<QuestBoard />)
    await waitFor(() => expect(screen.getAllByText(/50 XP/).length).toBeGreaterThanOrEqual(1))
  })

  it('renders gems reward', async () => {
    render(<QuestBoard />)
    await waitFor(() => expect(screen.getByText(/10.*gems?|10 💎/i)).toBeInTheDocument())
  })

  it('renders "Daily Quests" heading', async () => {
    render(<QuestBoard />)
    await waitFor(() => expect(screen.getByText('Daily Quests')).toBeInTheDocument())
  })
})

describe('QuestBoard — error state', () => {
  it('shows error message when fetch fails', async () => {
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: async () => ({}),
    } as unknown as Response)

    render(<QuestBoard />)
    await waitFor(() => expect(screen.getByText(/HTTP 500|Failed to load/i)).toBeInTheDocument())
  })

  it('shows error message when fetch throws', async () => {
    fetchMock.mockRejectedValueOnce(new Error('Network error'))
    render(<QuestBoard />)
    await waitFor(() => expect(screen.getByText(/Network error|Failed to load/i)).toBeInTheDocument())
  })
})

describe('QuestBoard — completed state', () => {
  it('shows all-complete message when every quest is done', async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ quests: [{ ...QUEST, completed: true, progress: 5 }] }),
    } as unknown as Response)

    render(<QuestBoard />)
    await waitFor(() => expect(screen.getByText(/All done|all complete|🎉/i)).toBeInTheDocument())
  })
})
