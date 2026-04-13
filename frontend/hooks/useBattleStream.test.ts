/**
 * @jest-environment jsdom
 */
import { act, renderHook } from '@testing-library/react'
import { applyEvent, useBattleStream, type BattleStreamState } from './useBattleStream'

// ── Mock WebSocket ────────────────────────────────────────────────────────────

/**
 * Minimal stub that mirrors the tiny WebSocket surface our hook uses.
 * Exposed as `global.WebSocket` in `beforeEach` and captured via
 * `lastSocket` so tests can drive events deterministically.
 */
class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  readyState: number = MockWebSocket.CONNECTING
  url: string
  onopen: ((ev: Event) => void) | null = null
  onmessage: ((ev: MessageEvent) => void) | null = null
  onerror: ((ev: Event) => void) | null = null
  onclose: ((ev: CloseEvent) => void) | null = null
  sent: string[] = []

  constructor(url: string) {
    this.url = url
    // record the most-recently-constructed instance for test assertions
    MockWebSocket.lastSocket = this
  }

  static lastSocket: MockWebSocket | null = null

  send(data: string) {
    this.sent.push(data)
  }

  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.(new CloseEvent('close'))
  }

  // Test helpers
  simulateOpen() {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.(new Event('open'))
  }
  simulateMessage(payload: unknown) {
    const data = typeof payload === 'string' ? payload : JSON.stringify(payload)
    this.onmessage?.(new MessageEvent('message', { data }))
  }
  simulateError() {
    this.onerror?.(new Event('error'))
  }
}

beforeEach(() => {
  MockWebSocket.lastSocket = null
  ;(globalThis as unknown as { WebSocket: unknown }).WebSocket = MockWebSocket
})

// ── applyEvent (pure) ─────────────────────────────────────────────────────────

describe('applyEvent', () => {
  const base: BattleStreamState = {
    bossHP: null,
    playerHP: null,
    lastDamage: null,
    combo: 0,
    round: 0,
  }

  it('hp event sets both HPs', () => {
    const next = applyEvent(base, { type: 'hp', boss: 80, player: 50 })
    expect(next.bossHP).toBe(80)
    expect(next.playerHP).toBe(50)
  })

  it('damage event populates lastDamage', () => {
    const next = applyEvent(base, { type: 'damage', amount: 12, target: 'boss' })
    expect(next.lastDamage).toEqual({ amount: 12, target: 'boss' })
  })

  it('combo event updates streak', () => {
    expect(applyEvent(base, { type: 'combo', count: 4 }).combo).toBe(4)
  })

  it('round event updates round counter', () => {
    expect(applyEvent(base, { type: 'round', round: 3 }).round).toBe(3)
  })
})

// ── useBattleStream (hook) ────────────────────────────────────────────────────

describe('useBattleStream', () => {
  it('does not open a socket when sessionId is null', () => {
    renderHook(() => useBattleStream(null))
    expect(MockWebSocket.lastSocket).toBeNull()
  })

  it('opens a socket to the expected URL once sessionId is provided', () => {
    renderHook(() => useBattleStream('s-1', { url: 'ws://test/boss' }))
    expect(MockWebSocket.lastSocket?.url).toBe('ws://test/boss/s-1/stream')
  })

  it('marks isConnected=true after onopen fires', () => {
    const { result } = renderHook(() => useBattleStream('s-1', { url: 'ws://test/boss' }))
    expect(result.current.isConnected).toBe(false)
    act(() => {
      MockWebSocket.lastSocket!.simulateOpen()
    })
    expect(result.current.isConnected).toBe(true)
  })

  it('reduces inbound hp/damage/combo/round events into battleState', () => {
    const { result } = renderHook(() => useBattleStream('s-1', { url: 'ws://test/boss' }))
    act(() => {
      MockWebSocket.lastSocket!.simulateOpen()
    })
    act(() => {
      MockWebSocket.lastSocket!.simulateMessage({ type: 'hp', boss: 70, player: 45 })
    })
    act(() => {
      MockWebSocket.lastSocket!.simulateMessage({
        type: 'damage',
        amount: 15,
        target: 'boss',
      })
    })
    act(() => {
      MockWebSocket.lastSocket!.simulateMessage({ type: 'combo', count: 2 })
    })
    act(() => {
      MockWebSocket.lastSocket!.simulateMessage({ type: 'round', round: 3 })
    })

    expect(result.current.battleState).toEqual({
      bossHP: 70,
      playerHP: 45,
      lastDamage: { amount: 15, target: 'boss' },
      combo: 2,
      round: 3,
    })
  })

  it('ignores unknown event types and malformed JSON without crashing', () => {
    const { result } = renderHook(() => useBattleStream('s-1', { url: 'ws://test/boss' }))
    act(() => {
      MockWebSocket.lastSocket!.simulateOpen()
      MockWebSocket.lastSocket!.simulateMessage('{not json')
      MockWebSocket.lastSocket!.simulateMessage({ type: 'ghost', foo: 1 })
      MockWebSocket.lastSocket!.simulateMessage(null)
    })
    expect(result.current.battleState.combo).toBe(0)
    expect(result.current.error).toBeNull()
  })

  it('sendAttack writes a framed attack message when socket is OPEN', () => {
    const { result } = renderHook(() => useBattleStream('s-1', { url: 'ws://test/boss' }))
    act(() => {
      MockWebSocket.lastSocket!.simulateOpen()
    })
    act(() => {
      result.current.sendAttack('A')
    })
    expect(MockWebSocket.lastSocket!.sent).toEqual([
      JSON.stringify({ type: 'attack', answer: 'A' }),
    ])
  })

  it('sendAttack is a no-op when the socket has not opened yet', () => {
    const { result } = renderHook(() => useBattleStream('s-1', { url: 'ws://test/boss' }))
    act(() => {
      result.current.sendAttack('B')
    })
    expect(MockWebSocket.lastSocket!.sent).toEqual([])
  })

  it('sets error and clears isConnected on socket error/close', () => {
    const { result } = renderHook(() => useBattleStream('s-1', { url: 'ws://test/boss' }))
    act(() => {
      MockWebSocket.lastSocket!.simulateOpen()
    })
    expect(result.current.isConnected).toBe(true)
    act(() => {
      MockWebSocket.lastSocket!.simulateError()
    })
    expect(result.current.error).toBe('battle stream error')
    act(() => {
      MockWebSocket.lastSocket!.close()
    })
    expect(result.current.isConnected).toBe(false)
  })

  it('closes the socket on unmount', () => {
    const { unmount } = renderHook(() => useBattleStream('s-1', { url: 'ws://test/boss' }))
    const ws = MockWebSocket.lastSocket!
    act(() => {
      ws.simulateOpen()
    })
    unmount()
    expect(ws.readyState).toBe(MockWebSocket.CLOSED)
  })
})
