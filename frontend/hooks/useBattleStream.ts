'use client'

/**
 * useBattleStream — WebSocket hook for real-time boss-battle updates (tl-bfo).
 *
 * Opens a WebSocket to the gaming-service battle stream for the given session,
 * parses incoming event frames, and exposes a reducer-like snapshot of the
 * battle state plus an imperative `sendAttack` action.
 *
 * Event protocol (server → client, one JSON object per message):
 *   { "type": "hp",     "boss": number, "player": number }
 *   { "type": "damage", "amount": number, "target": "boss" | "player" }
 *   { "type": "combo",  "count": number }
 *   { "type": "round",  "round": number }
 *
 * Action protocol (client → server):
 *   { "type": "attack", "answer": string }
 *
 * Unknown event types are ignored (forward-compatible). Malformed JSON is
 * logged to the console and dropped.
 */

import { useCallback, useEffect, useRef, useState } from 'react'

/** Snapshot of the live battle state as streamed from the server. */
export interface BattleStreamState {
  /** Current boss HP, or null until the first `hp` event arrives. */
  bossHP: number | null
  /** Current player HP, or null until the first `hp` event arrives. */
  playerHP: number | null
  /**
   * Last damage event payload — `{ amount, target }` or null when no damage
   * event has been received yet. Consumers animate on identity change.
   */
  lastDamage: { amount: number; target: 'boss' | 'player' } | null
  /** Current answer combo streak. Defaults to 0. */
  combo: number
  /** Current round number. Defaults to 0 until the first `round` event. */
  round: number
}

/** Public return shape for useBattleStream. */
export interface BattleStreamHandle {
  /** Latest battle snapshot. Replaced on every inbound event. */
  battleState: BattleStreamState
  /** Send an attack frame upstream. No-op if the socket is not open. */
  sendAttack: (answer: string) => void
  /** True once the WebSocket has fired `open` and not yet closed/errored. */
  isConnected: boolean
  /** Last error message from the stream, or null if healthy. */
  error: string | null
}

/** Known server event type names. Unknown types are dropped. */
type BattleEvent =
  | { type: 'hp'; boss: number; player: number }
  | { type: 'damage'; amount: number; target: 'boss' | 'player' }
  | { type: 'combo'; count: number }
  | { type: 'round'; round: number }

const INITIAL_STATE: BattleStreamState = {
  bossHP: null,
  playerHP: null,
  lastDamage: null,
  combo: 0,
  round: 0,
}

/** Base URL for the gaming-service battle stream. */
const DEFAULT_BASE_URL = process.env.NEXT_PUBLIC_GAMING_WS_URL ?? 'ws://localhost:8083/gaming/boss'

/**
 * Apply a single parsed event to the current battle state.
 *
 * Pure function for testability. Returns the original state object when the
 * event is unrecognised so React can skip re-renders.
 */
export function applyEvent(state: BattleStreamState, ev: BattleEvent): BattleStreamState {
  switch (ev.type) {
    case 'hp':
      return { ...state, bossHP: ev.boss, playerHP: ev.player }
    case 'damage':
      return { ...state, lastDamage: { amount: ev.amount, target: ev.target } }
    case 'combo':
      return { ...state, combo: ev.count }
    case 'round':
      return { ...state, round: ev.round }
    default:
      return state
  }
}

/**
 * Open a WebSocket to the gaming-service battle stream for `sessionId`.
 *
 * Returns a stable handle whose `battleState` snapshot updates as server
 * events arrive. Passing `null` for `sessionId` tears down any open socket
 * (useful when the component hasn't started a battle yet).
 *
 * @param sessionId - Gaming-service battle session id, or null to stay idle.
 * @param options   - Optional overrides (custom `url`, typically for tests).
 * @returns Battle stream handle with state, sender, connected flag, error.
 */
export function useBattleStream(
  sessionId: string | null,
  options?: { url?: string },
): BattleStreamHandle {
  const [battleState, setBattleState] = useState<BattleStreamState>(INITIAL_STATE)
  const [isConnected, setIsConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const socketRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    if (!sessionId) {
      setIsConnected(false)
      return
    }

    const base = options?.url ?? DEFAULT_BASE_URL
    const url = `${base}/${sessionId}/stream`
    const ws = new WebSocket(url)
    socketRef.current = ws

    ws.onopen = () => {
      setIsConnected(true)
      setError(null)
    }

    ws.onmessage = (msg: MessageEvent) => {
      let parsed: unknown
      try {
        parsed = JSON.parse(typeof msg.data === 'string' ? msg.data : '')
      } catch (e) {
        console.error('useBattleStream: malformed JSON from stream', e)
        return
      }
      if (!parsed || typeof parsed !== 'object' || !('type' in parsed)) return
      const ev = parsed as BattleEvent
      setBattleState((prev) => applyEvent(prev, ev))
    }

    ws.onerror = () => {
      setError('battle stream error')
    }

    ws.onclose = () => {
      setIsConnected(false)
    }

    return () => {
      ws.onopen = null
      ws.onmessage = null
      ws.onerror = null
      ws.onclose = null
      // Only close sockets we actually opened — guard against readyState
      // to avoid the "WebSocket is closed before the connection is
      // established" console warning in tests and StrictMode double-effects.
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close()
      }
      socketRef.current = null
      setIsConnected(false)
    }
  }, [sessionId, options?.url])

  const sendAttack = useCallback((answer: string) => {
    const ws = socketRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    ws.send(JSON.stringify({ type: 'attack', answer }))
  }, [])

  return { battleState, sendAttack, isConnected, error }
}
