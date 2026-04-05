/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import PowerUpShop from './PowerUpShop'

const MOCK_CATALOG = {
  items: [
    {
      type: 'shield',
      label: 'Shield',
      icon: '🛡️',
      description: 'Blocks damage for 3 turns.',
      gem_cost: 2,
    },
    {
      type: 'double_damage',
      label: 'Double Damage',
      icon: '⚔️',
      description: 'Doubles damage for 2 turns.',
      gem_cost: 3,
    },
    {
      type: 'heal',
      label: 'Heal',
      icon: '💊',
      description: 'Restores 30 HP.',
      gem_cost: 2,
    },
    {
      type: 'critical',
      label: 'Critical Hit',
      icon: '💥',
      description: 'Guarantees a critical hit.',
      gem_cost: 5,
    },
  ],
}

const DEFAULT_PROPS = {
  userId: 'user-abc',
  initialGems: 10,
  initialInventory: {},
}

let fetchMock: jest.Mock

beforeEach(() => {
  fetchMock = jest.fn().mockResolvedValue({
    ok: true,
    json: async () => MOCK_CATALOG,
  } as unknown as Response)
  global.fetch = fetchMock
})

afterEach(() => {
  jest.restoreAllMocks()
})

// ── Rendering ─────────────────────────────────────────────────────────────────

describe('PowerUpShop — rendering', () => {
  it('fetches the catalog from /api/gaming/shop on mount', async () => {
    render(<PowerUpShop {...DEFAULT_PROPS} />)
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('/api/gaming/shop'))
  })

  it('displays all four catalog items after load', async () => {
    render(<PowerUpShop {...DEFAULT_PROPS} />)
    await waitFor(() => expect(screen.getByText('Shield')).toBeInTheDocument())
    expect(screen.getByText('Double Damage')).toBeInTheDocument()
    expect(screen.getByText('Heal')).toBeInTheDocument()
    expect(screen.getByText('Critical Hit')).toBeInTheDocument()
  })

  it('shows the initial gem balance', async () => {
    render(<PowerUpShop {...DEFAULT_PROPS} initialGems={7} />)
    await waitFor(() => expect(screen.getByText('Shield')).toBeInTheDocument())
    expect(screen.getByText('7')).toBeInTheDocument()
  })

  it('shows inventory count badge when initialInventory is non-zero', async () => {
    render(<PowerUpShop {...DEFAULT_PROPS} initialInventory={{ shield: 3 }} />)
    await waitFor(() => expect(screen.getByText('Shield')).toBeInTheDocument())
    expect(screen.getByText('×3')).toBeInTheDocument()
  })

  it('renders an error when catalog fetch fails', async () => {
    fetchMock.mockRejectedValueOnce(new Error('network'))
    render(<PowerUpShop {...DEFAULT_PROPS} />)
    await waitFor(() =>
      expect(screen.getByText(/Failed to load shop catalog/i)).toBeInTheDocument(),
    )
  })
})

// ── Affordability ──────────────────────────────────────────────────────────────

describe('PowerUpShop — affordability', () => {
  it('disables buy button when player cannot afford the item', async () => {
    render(<PowerUpShop {...DEFAULT_PROPS} initialGems={1} />)
    await waitFor(() => expect(screen.getByText('Shield')).toBeInTheDocument())

    // All items cost ≥ 2; with 1 gem all Buy buttons should be disabled.
    const buyButtons = screen.getAllByRole('button', { name: /Buy/i })
    for (const btn of buyButtons) {
      expect(btn).toBeDisabled()
    }
  })

  it('enables buy button when player can afford the item', async () => {
    render(<PowerUpShop {...DEFAULT_PROPS} initialGems={10} />)
    await waitFor(() => expect(screen.getByText('Shield')).toBeInTheDocument())

    const shieldBtn = screen.getByRole('button', { name: /Buy Shield/i })
    expect(shieldBtn).not.toBeDisabled()
  })
})

// ── Purchase flow ─────────────────────────────────────────────────────────────

describe('PowerUpShop — purchase', () => {
  it('calls POST /api/gaming/shop with correct payload on buy', async () => {
    const buyResp = { power_up: 'shield', new_count: 1, gems_left: 8 }
    fetchMock
      .mockResolvedValueOnce({ ok: true, json: async () => MOCK_CATALOG } as unknown as Response)
      .mockResolvedValueOnce({ ok: true, json: async () => buyResp } as unknown as Response)

    render(<PowerUpShop {...DEFAULT_PROPS} />)
    await waitFor(() => expect(screen.getByText('Shield')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: /Buy Shield/i }))

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/gaming/shop',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ user_id: 'user-abc', power_up: 'shield' }),
        }),
      ),
    )
  })

  it('updates gem balance after successful purchase', async () => {
    const buyResp = { power_up: 'shield', new_count: 1, gems_left: 8 }
    fetchMock
      .mockResolvedValueOnce({ ok: true, json: async () => MOCK_CATALOG } as unknown as Response)
      .mockResolvedValueOnce({ ok: true, json: async () => buyResp } as unknown as Response)

    render(<PowerUpShop {...DEFAULT_PROPS} initialGems={10} />)
    await waitFor(() => expect(screen.getByText('Shield')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: /Buy Shield/i }))

    await waitFor(() => expect(screen.getByText('8')).toBeInTheDocument())
  })

  it('increments inventory count after successful purchase', async () => {
    const buyResp = { power_up: 'shield', new_count: 2, gems_left: 8 }
    fetchMock
      .mockResolvedValueOnce({ ok: true, json: async () => MOCK_CATALOG } as unknown as Response)
      .mockResolvedValueOnce({ ok: true, json: async () => buyResp } as unknown as Response)

    render(<PowerUpShop {...DEFAULT_PROPS} initialInventory={{ shield: 1 }} />)
    await waitFor(() => expect(screen.getByText('Shield')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: /Buy Shield/i }))

    await waitFor(() => expect(screen.getByText('×2')).toBeInTheDocument())
  })

  it('shows an error message when the purchase API fails', async () => {
    fetchMock
      .mockResolvedValueOnce({ ok: true, json: async () => MOCK_CATALOG } as unknown as Response)
      .mockResolvedValueOnce({
        ok: false,
        json: async () => ({ error: 'not enough gems' }),
      } as unknown as Response)

    render(<PowerUpShop {...DEFAULT_PROPS} />)
    await waitFor(() => expect(screen.getByText('Shield')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: /Buy Shield/i }))

    await waitFor(() => expect(screen.getByText(/not enough gems/i)).toBeInTheDocument())
  })

  it('shows an error on network failure during purchase', async () => {
    fetchMock
      .mockResolvedValueOnce({ ok: true, json: async () => MOCK_CATALOG } as unknown as Response)
      .mockRejectedValueOnce(new Error('network'))

    render(<PowerUpShop {...DEFAULT_PROPS} />)
    await waitFor(() => expect(screen.getByText('Shield')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: /Buy Shield/i }))

    await waitFor(() => expect(screen.getByText(/Network error/i)).toBeInTheDocument())
  })
})
