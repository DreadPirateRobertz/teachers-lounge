/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import BossProgressionMap, { type ProgressionData } from './BossProgressionMap'

// ── Fixtures ──────────────────────────────────────────────────────────────────

const FRESH_USER: ProgressionData = {
  total_defeated: 0,
  nodes: [
    {
      boss_id: 'the_atom',
      name: 'THE ATOM',
      topic: 'Atomic Structure',
      tier: 1,
      victory_xp: 500,
      primary_color: '#00aaff',
      state: 'current',
    },
    {
      boss_id: 'the_bonder',
      name: 'THE BONDER',
      topic: 'Chemical Bonding',
      tier: 2,
      victory_xp: 750,
      primary_color: '#00ff88',
      state: 'locked',
    },
    {
      boss_id: 'final_boss',
      name: 'FINAL BOSS',
      topic: 'Advanced Organic',
      tier: 6,
      victory_xp: 3000,
      primary_color: '#ff0055',
      state: 'locked',
    },
  ],
}

const ONE_DEFEATED: ProgressionData = {
  total_defeated: 1,
  nodes: [
    {
      boss_id: 'the_atom',
      name: 'THE ATOM',
      topic: 'Atomic Structure',
      tier: 1,
      victory_xp: 500,
      primary_color: '#00aaff',
      state: 'defeated',
    },
    {
      boss_id: 'the_bonder',
      name: 'THE BONDER',
      topic: 'Chemical Bonding',
      tier: 2,
      victory_xp: 750,
      primary_color: '#00ff88',
      state: 'current',
    },
    {
      boss_id: 'final_boss',
      name: 'FINAL BOSS',
      topic: 'Advanced Organic',
      tier: 6,
      victory_xp: 3000,
      primary_color: '#ff0055',
      state: 'locked',
    },
  ],
}

const ALL_DEFEATED: ProgressionData = {
  total_defeated: 3,
  nodes: [
    {
      boss_id: 'the_atom',
      name: 'THE ATOM',
      topic: 'Atomic Structure',
      tier: 1,
      victory_xp: 500,
      primary_color: '#00aaff',
      state: 'defeated',
    },
    {
      boss_id: 'the_bonder',
      name: 'THE BONDER',
      topic: 'Chemical Bonding',
      tier: 2,
      victory_xp: 750,
      primary_color: '#00ff88',
      state: 'defeated',
    },
    {
      boss_id: 'final_boss',
      name: 'FINAL BOSS',
      topic: 'Advanced Organic',
      tier: 6,
      victory_xp: 3000,
      primary_color: '#ff0055',
      state: 'defeated',
    },
  ],
}

// ── Test setup ────────────────────────────────────────────────────────────────

let fetchMock: jest.Mock

function mockFetch(data: ProgressionData) {
  fetchMock = jest.fn().mockResolvedValue({
    ok: true,
    json: async () => data,
  } as unknown as Response)
  global.fetch = fetchMock
}

afterEach(() => jest.restoreAllMocks())

// ── Loading state ─────────────────────────────────────────────────────────────

describe('BossProgressionMap — loading', () => {
  it('shows loading skeleton before fetch resolves', () => {
    fetchMock = jest.fn().mockReturnValue(new Promise(() => {})) // never resolves
    global.fetch = fetchMock
    render(<BossProgressionMap />)
    expect(screen.getByLabelText('Loading boss progression')).toBeInTheDocument()
  })

  it('calls the correct API endpoint on mount', async () => {
    mockFetch(FRESH_USER)
    render(<BossProgressionMap />)
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/gaming/progression',
        expect.objectContaining({ cache: 'no-store' }),
      ),
    )
  })

  it('uses a custom apiUrl prop when provided', async () => {
    mockFetch(FRESH_USER)
    render(<BossProgressionMap apiUrl="/custom/endpoint" />)
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith('/custom/endpoint', expect.anything()),
    )
  })
})

// ── Error state ───────────────────────────────────────────────────────────────

describe('BossProgressionMap — error', () => {
  it('shows error message when fetch rejects', async () => {
    fetchMock = jest.fn().mockRejectedValue(new Error('network'))
    global.fetch = fetchMock
    render(<BossProgressionMap />)
    await waitFor(() =>
      expect(screen.getByText(/Failed to load boss progression/i)).toBeInTheDocument(),
    )
  })

  it('shows error message when server returns non-ok status', async () => {
    fetchMock = jest.fn().mockResolvedValue({ ok: false, status: 500 } as unknown as Response)
    global.fetch = fetchMock
    render(<BossProgressionMap />)
    await waitFor(() =>
      expect(screen.getByText(/Failed to load boss progression/i)).toBeInTheDocument(),
    )
  })
})

// ── Fresh user rendering ──────────────────────────────────────────────────────

describe('BossProgressionMap — fresh user', () => {
  beforeEach(() => mockFetch(FRESH_USER))

  it('renders all boss names', async () => {
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByText('THE ATOM')).toBeInTheDocument())
    expect(screen.getByText('THE BONDER')).toBeInTheDocument()
    expect(screen.getByText('FINAL BOSS')).toBeInTheDocument()
  })

  it('shows defeat count summary', async () => {
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByText('THE ATOM')).toBeInTheDocument())
    expect(screen.getByText('0')).toBeInTheDocument()
  })

  it('tier-1 boss shows FIGHT CTA', async () => {
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByText('THE ATOM')).toBeInTheDocument())
    expect(screen.getByText('FIGHT')).toBeInTheDocument()
  })

  it('locked bosses do not show FIGHT CTA', async () => {
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByText('THE BONDER')).toBeInTheDocument())
    // Only one FIGHT CTA should be present for the current boss
    expect(screen.getAllByText('FIGHT')).toHaveLength(1)
  })
})

// ── One defeated ─────────────────────────────────────────────────────────────

describe('BossProgressionMap — one defeated', () => {
  beforeEach(() => mockFetch(ONE_DEFEATED))

  it('defeated boss shows XP reward label', async () => {
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByText('THE ATOM')).toBeInTheDocument())
    expect(screen.getByText('+500 XP')).toBeInTheDocument()
  })

  it('current boss (tier 2) shows FIGHT CTA', async () => {
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByText('THE BONDER')).toBeInTheDocument())
    expect(screen.getByText('FIGHT')).toBeInTheDocument()
  })

  it('defeated boss links to boss-battle page', async () => {
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByText('THE ATOM')).toBeInTheDocument())
    const links = screen.getAllByRole('link')
    const battleLinks = links.filter((l) =>
      l.getAttribute('href')?.includes('/boss-battle/the_atom'),
    )
    expect(battleLinks.length).toBeGreaterThan(0)
  })

  it('current boss links to boss-battle page', async () => {
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByText('THE BONDER')).toBeInTheDocument())
    const links = screen.getAllByRole('link')
    const battleLinks = links.filter((l) =>
      l.getAttribute('href')?.includes('/boss-battle/the_bonder'),
    )
    expect(battleLinks.length).toBeGreaterThan(0)
  })

  it('summary shows 1 defeated', async () => {
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByText('THE ATOM')).toBeInTheDocument())
    expect(screen.getByText('1')).toBeInTheDocument()
  })
})

// ── All defeated ──────────────────────────────────────────────────────────────

describe('BossProgressionMap — all defeated', () => {
  beforeEach(() => mockFetch(ALL_DEFEATED))

  it('no FIGHT CTA shown when all bosses are defeated', async () => {
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByText('THE ATOM')).toBeInTheDocument())
    expect(screen.queryByText('FIGHT')).not.toBeInTheDocument()
  })

  it('all XP rewards shown', async () => {
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByText('+500 XP')).toBeInTheDocument())
    expect(screen.getByText('+750 XP')).toBeInTheDocument()
    expect(screen.getByText('+3000 XP')).toBeInTheDocument()
  })

  it('summary shows correct total', async () => {
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByText('+500 XP')).toBeInTheDocument())
    expect(screen.getByText('3')).toBeInTheDocument()
  })
})

// ── Accessibility ─────────────────────────────────────────────────────────────

describe('BossProgressionMap — accessibility', () => {
  it('trail container has list role', async () => {
    mockFetch(FRESH_USER)
    render(<BossProgressionMap />)
    await waitFor(() =>
      expect(screen.getByRole('list', { name: /Boss progression trail/i })).toBeInTheDocument(),
    )
  })

  it('each node has listitem role with descriptive label', async () => {
    mockFetch(ONE_DEFEATED)
    render(<BossProgressionMap />)
    await waitFor(() => expect(screen.getByLabelText(/THE ATOM — defeated/i)).toBeInTheDocument())
    expect(screen.getByLabelText(/THE BONDER — current/i)).toBeInTheDocument()
  })
})
