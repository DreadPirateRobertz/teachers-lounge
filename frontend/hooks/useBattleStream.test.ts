/**
 * @jest-environment jsdom
 *
 * Tests for useBattleStream hook.
 *
 * Covers: initial state, connection lifecycle (open/close/error),
 * message parsing, sendAttack, cleanup on unmount, and sessionId changes.
 */
import { renderHook, act } from '@testing-library/react'
import { useBattleStream, type BattleEvent } from './useBattleStream'

// ─── WebSocket mock ────────────────────────────────────────────────────────────

/**
 * Minimal WebSocket mock that tracks instances and simulates server-side
 * lifecycle events.  Replaced in `global.WebSocket` before each test.
 */
class MockWebSocket {
  /** All instances created during the current test. */
  static instances: MockWebSocket[] = []

  // Standard WebSocket ready-state constants.
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3

  readonly url: string
  readyState: number = MockWebSocket.CONNECTING

  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null

  /** Messages sent via `.send()` while the socket is open. */
  readonly sentMessages: string[] = []

  private _closed = false

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  send(data: string): void {
    if (this.readyState === MockWebSocket.OPEN) {
      this.sentMessages.push(data)
    }
  }

  close(): void {
    if (this._closed) return
    this._closed = true
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({} as CloseEvent)
  }

  // ── Test helpers — simulate server-side events ──────────────────────────────

  /**
   * Simulate the server accepting the connection.
   */
  simulateOpen(): void {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.({} as Event)
  }

  /**
   * Simulate a server-pushed battle event message.
   *
   * @param data - BattleEvent object that will be JSON-serialised and delivered.
   */
  simulateMessage(data: BattleEvent): void {
    this.onmessage?.({ data: JSON.stringify(data) } as MessageEvent)
  }

  /**
   * Simulate the server closing the connection.
   */
  simulateClose(): void {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({} as CloseEvent)
  }

  /**
   * Simulate a socket error.
   */
  simulateError(): void {
    this.onerror?.({} as Event)
  }
}

// ─── Fixtures ──────────────────────────────────────────────────────────────────

/**
 * Build a BattleEvent with sensible defaults, optionally overridden.
 *
 * @param overrides - Partial BattleEvent fields to override.
 * @returns A complete BattleEvent suitable for testing.
 */
function makeEvent(overrides: Partial<BattleEvent> = {}): BattleEvent {
  return {
    type: 'state_update',
    boss_hp: 80,
    player_hp: 90,
    damage_dealt: 10,
    combo_count: 2,
    phase: 'active',
    ...overrides,
  }
}

// ─── Setup / teardown ─────────────────────────────────────────────────────────

beforeEach(() => {
  MockWebSocket.instances = []
  global.WebSocket = MockWebSocket as unknown as typeof WebSocket
})

afterEach(() => {
  jest.clearAllMocks()
})

// ─── Tests ────────────────────────────────────────────────────────────────────

describe('useBattleStream — initial state', () => {
  it('returns null battleState and isConnected=false before any connection', () => {
    const { result } = renderHook(() => useBattleStream(null))
    expect(result.current.battleState).toBeNull()
    expect(result.current.isConnected).toBe(false)
  })

  it('does not open a WebSocket when sessionId is null', () => {
    renderHook(() => useBattleStream(null))
    expect(MockWebSocket.instances).toHaveLength(0)
  })
})

describe('useBattleStream — connection lifecycle', () => {
  it('opens a WebSocket for the correct stream URL', () => {
    renderHook(() => useBattleStream('session-123'))
    expect(MockWebSocket.instances).toHaveLength(1)
    expect(MockWebSocket.instances[0].url).toContain('/gaming/boss/stream/session-123')
  })

  it('uses a ws:// scheme (not http://)', () => {
    renderHook(() => useBattleStream('session-123'))
    expect(MockWebSocket.instances[0].url).toMatch(/^ws:\/\//)
  })

  it('sets isConnected=true on socket open', () => {
    const { result } = renderHook(() => useBattleStream('session-123'))
    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })
    expect(result.current.isConnected).toBe(true)
  })

  it('sets isConnected=false on socket close', () => {
    const { result } = renderHook(() => useBattleStream('session-123'))
    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })
    act(() => {
      MockWebSocket.instances[0].simulateClose()
    })
    expect(result.current.isConnected).toBe(false)
  })

  it('sets isConnected=false on socket error', () => {
    const { result } = renderHook(() => useBattleStream('session-123'))
    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })
    act(() => {
      MockWebSocket.instances[0].simulateError()
    })
    expect(result.current.isConnected).toBe(false)
  })
})

describe('useBattleStream — message handling', () => {
  it('updates battleState on a valid battle event', () => {
    const { result } = renderHook(() => useBattleStream('session-abc'))
    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })
    act(() => {
      MockWebSocket.instances[0].simulateMessage(makeEvent())
    })
    expect(result.current.battleState).toEqual({
      bossHp: 80,
      playerHp: 90,
      damageDealt: 10,
      comboCount: 2,
      phase: 'active',
    })
  })

  it('reflects boss_hp=0 and phase=victory on a killing blow event', () => {
    const { result } = renderHook(() => useBattleStream('session-abc'))
    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })
    act(() => {
      MockWebSocket.instances[0].simulateMessage(
        makeEvent({ boss_hp: 0, damage_dealt: 30, phase: 'victory' }),
      )
    })
    expect(result.current.battleState?.bossHp).toBe(0)
    expect(result.current.battleState?.phase).toBe('victory')
    expect(result.current.battleState?.damageDealt).toBe(30)
  })

  it('reflects phase=defeat when player hp reaches 0', () => {
    const { result } = renderHook(() => useBattleStream('session-abc'))
    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })
    act(() => {
      MockWebSocket.instances[0].simulateMessage(
        makeEvent({ player_hp: 0, phase: 'defeat' }),
      )
    })
    expect(result.current.battleState?.phase).toBe('defeat')
    expect(result.current.battleState?.playerHp).toBe(0)
  })

  it('updates comboCount from the stream', () => {
    const { result } = renderHook(() => useBattleStream('session-abc'))
    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })
    act(() => {
      MockWebSocket.instances[0].simulateMessage(makeEvent({ combo_count: 7 }))
    })
    expect(result.current.battleState?.comboCount).toBe(7)
  })

  it('silently ignores malformed JSON messages', () => {
    const { result } = renderHook(() => useBattleStream('session-abc'))
    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })
    act(() => {
      MockWebSocket.instances[0].onmessage?.({ data: 'not-valid-json' } as MessageEvent)
    })
    // battleState must remain null — no crash, no partial state
    expect(result.current.battleState).toBeNull()
  })

  it('preserves the last valid state after a malformed message', () => {
    const { result } = renderHook(() => useBattleStream('session-abc'))
    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })
    act(() => {
      MockWebSocket.instances[0].simulateMessage(makeEvent({ boss_hp: 60 }))
    })
    act(() => {
      MockWebSocket.instances[0].onmessage?.({ data: '{{bad}}' } as MessageEvent)
    })
    expect(result.current.battleState?.bossHp).toBe(60)
  })
})

describe('useBattleStream — sendAttack', () => {
  it('sends a well-formed attack message when connected', () => {
    const { result } = renderHook(() => useBattleStream('session-xyz'))
    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })
    act(() => {
      result.current.sendAttack({
        sessionId: 'session-xyz',
        answerCorrect: true,
        baseDamage: 20,
      })
    })
    const msgs = MockWebSocket.instances[0].sentMessages
    expect(msgs).toHaveLength(1)
    expect(JSON.parse(msgs[0])).toMatchObject({
      type: 'attack',
      session_id: 'session-xyz',
      answer_correct: true,
      base_damage: 20,
    })
  })

  it('is a no-op when the socket is still connecting (not yet OPEN)', () => {
    const { result } = renderHook(() => useBattleStream('session-xyz'))
    // Deliberately do NOT call simulateOpen — socket is CONNECTING
    act(() => {
      result.current.sendAttack({
        sessionId: 'session-xyz',
        answerCorrect: false,
        baseDamage: 10,
      })
    })
    expect(MockWebSocket.instances[0].sentMessages).toHaveLength(0)
  })

  it('is a no-op (no throw) when sessionId is null', () => {
    const { result } = renderHook(() => useBattleStream(null))
    expect(() => {
      act(() => {
        result.current.sendAttack({ sessionId: 'x', answerCorrect: false, baseDamage: 5 })
      })
    }).not.toThrow()
  })

  it('encodes answerCorrect=false correctly', () => {
    const { result } = renderHook(() => useBattleStream('session-xyz'))
    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })
    act(() => {
      result.current.sendAttack({
        sessionId: 'session-xyz',
        answerCorrect: false,
        baseDamage: 5,
      })
    })
    const parsed = JSON.parse(MockWebSocket.instances[0].sentMessages[0])
    expect(parsed.answer_correct).toBe(false)
  })
})

describe('useBattleStream — cleanup', () => {
  it('closes the WebSocket on unmount', () => {
    const { unmount } = renderHook(() => useBattleStream('session-abc'))
    const ws = MockWebSocket.instances[0]
    unmount()
    expect(ws.readyState).toBe(MockWebSocket.CLOSED)
  })

  it('opens a new WebSocket when sessionId changes', () => {
    const { rerender } = renderHook(
      ({ sid }: { sid: string | null }) => useBattleStream(sid),
      { initialProps: { sid: 'session-1' as string | null } },
    )
    expect(MockWebSocket.instances).toHaveLength(1)
    expect(MockWebSocket.instances[0].url).toContain('session-1')

    rerender({ sid: 'session-2' })
    expect(MockWebSocket.instances).toHaveLength(2)
    expect(MockWebSocket.instances[1].url).toContain('session-2')
  })

  it('closes the old WebSocket when sessionId changes', () => {
    const { rerender } = renderHook(
      ({ sid }: { sid: string | null }) => useBattleStream(sid),
      { initialProps: { sid: 'session-1' as string | null } },
    )
    const firstWs = MockWebSocket.instances[0]
    rerender({ sid: 'session-2' })
    expect(firstWs.readyState).toBe(MockWebSocket.CLOSED)
  })
})
