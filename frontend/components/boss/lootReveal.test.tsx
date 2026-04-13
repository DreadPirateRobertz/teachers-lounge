/**
 * @jest-environment jsdom
 */
import { act, render, screen } from '@testing-library/react'
import LootReveal, { type LootItem } from './LootReveal'

jest.useFakeTimers()

const items: LootItem[] = [
  { key: 'gems', icon: '💎', label: 'Gems', amount: 25 },
  { key: 'xp', icon: '⭐', label: 'XP', amount: 120 },
  { key: 'badge', icon: '🏅', label: 'Badge: Atom Slayer', amount: null },
]

describe('LootReveal', () => {
  afterEach(() => {
    jest.clearAllTimers()
  })

  it('renders all loot rows with accessible region label', () => {
    render(<LootReveal items={items} staggerMs={100} />)
    expect(screen.getByRole('region', { name: /battle rewards/i })).toBeInTheDocument()
    for (const it of items) {
      expect(screen.getByTestId(`loot-row-${it.key}`)).toBeInTheDocument()
    }
  })

  it('reveals rows in order on each stagger tick', () => {
    render(<LootReveal items={items} staggerMs={100} />)
    expect(screen.getByTestId('loot-row-gems')).toHaveAttribute('aria-hidden', 'true')

    act(() => {
      jest.advanceTimersByTime(100)
    })
    expect(screen.getByTestId('loot-row-gems')).toHaveAttribute('aria-hidden', 'false')
    expect(screen.getByTestId('loot-row-xp')).toHaveAttribute('aria-hidden', 'true')

    act(() => {
      jest.advanceTimersByTime(100)
    })
    expect(screen.getByTestId('loot-row-xp')).toHaveAttribute('aria-hidden', 'false')
  })

  it('invokes onContinue after the last row reveals', () => {
    const onContinue = jest.fn()
    render(<LootReveal items={items} onContinue={onContinue} staggerMs={50} />)

    // Not yet called before any row reveals.
    expect(onContinue).not.toHaveBeenCalled()

    // Advance one tick per row. Each setTimeout is scheduled by the effect
    // that ran after the previous state update, so we advance them one at a
    // time for React to re-run the effect between ticks.
    for (let i = 0; i < items.length; i++) {
      act(() => {
        jest.advanceTimersByTime(50)
      })
    }
    expect(onContinue).toHaveBeenCalledTimes(1)
  })

  it('omits amount for qualitative rewards', () => {
    render(<LootReveal items={items} staggerMs={50} />)
    const badgeRow = screen.getByTestId('loot-row-badge')
    expect(badgeRow.textContent).toContain('Badge: Atom Slayer')
    expect(badgeRow.textContent).not.toMatch(/\+\d/)
  })

  it('fires onContinue exactly once even when parent re-renders with a new callback identity', () => {
    const onContinue = jest.fn()
    const { rerender } = render(<LootReveal items={items} onContinue={onContinue} staggerMs={50} />)
    for (let i = 0; i < items.length; i++) {
      act(() => {
        jest.advanceTimersByTime(50)
      })
    }
    expect(onContinue).toHaveBeenCalledTimes(1)

    // Simulate a parent re-render that passes a NEW inline function ref.
    // The effect re-runs (onContinue is in its dep array) but the useRef
    // guard in LootReveal must suppress a second invocation.
    const onContinue2 = jest.fn()
    rerender(<LootReveal items={items} onContinue={onContinue2} staggerMs={50} />)
    act(() => {
      jest.advanceTimersByTime(0)
    })
    expect(onContinue).toHaveBeenCalledTimes(1)
    expect(onContinue2).not.toHaveBeenCalled()
  })

  it('handles empty loot list gracefully', () => {
    const onContinue = jest.fn()
    render(<LootReveal items={[]} onContinue={onContinue} staggerMs={50} />)
    act(() => {
      jest.advanceTimersByTime(50)
    })
    expect(onContinue).toHaveBeenCalledTimes(1)
  })
})
