'use client'

import { useCallback, useEffect, useRef, useState } from 'react'

/** Gaming-service base URL — same env var used across the frontend. */
const GAMING_API = process.env.NEXT_PUBLIC_GAMING_SERVICE_URL ?? 'http://localhost:8083'

/**
 * Convert an HTTP/HTTPS URL to the corresponding WS/WSS URL.
 *
 * @param httpUrl - URL starting with `http://` or `https://`.
 * @returns The same URL with the scheme replaced by `ws://` or `wss://`.
 */
function toWsUrl(httpUrl: string): string {
  return httpUrl.replace(/^https?:\/\//, (match) => (match === 'https://' ? 'wss://' : 'ws://'))
}

// ─── Public types ─────────────────────────────────────────────────────────────

/** Battle phase reported by the gaming-service. */
export type BattlePhase = 'active' | 'victory' | 'defeat'

/**
 * Raw event shape pushed by the gaming-service WebSocket stream.
 * Field names follow the existing snake_case REST convention.
 */
export interface BattleEvent {
  /** Discriminator — used for routing on the server but surfaced here for completeness. */
  type: 'state_update' | 'attack_result' | 'battle_end' | 'error'
  /** Current boss hit-points. */
  boss_hp: number
  /** Current player hit-points. */
  player_hp: number
  /** Damage dealt on the most recent attack turn. */
  damage_dealt: number
  /** Running combo count (consecutive correct answers). */
  combo_count: number
  /** Current battle phase. */
  phase: BattlePhase
}

/** Camel-cased battle snapshot exposed to consuming components. */
export interface BattleState {
  /** Current boss hit-points. */
  bossHp: number
  /** Current player hit-points. */
  playerHp: number
  /** Damage dealt on the most recent attack turn. */
  damageDealt: number
  /** Running combo count (consecutive correct answers). */
  comboCount: number
  /** Current battle phase. */
  phase: BattlePhase
}

/** Payload for {@link UseBattleStreamResult.sendAttack}. */
export interface SendAttackPayload {
  /** Active battle session ID (from `POST /gaming/boss/start`). */
  sessionId: string
  /** Whether the player's answer was correct. */
  answerCorrect: boolean
  /** Base damage calculated from question difficulty. */
  baseDamage: number
}

/** Return value of {@link useBattleStream}. */
export interface UseBattleStreamResult {
  /**
   * Latest battle state received from the WebSocket stream, or `null` if no
   * message has arrived yet.
   */
  battleState: BattleState | null
  /**
   * Send an attack event to the gaming-service over the WebSocket.
   * No-op when the socket is not in the `OPEN` ready-state.
   */
  sendAttack: (payload: SendAttackPayload) => void
  /** `true` while the WebSocket is in the `OPEN` ready-state. */
  isConnected: boolean
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

/**
 * Connect to the gaming-service WebSocket battle stream and expose realtime
 * battle state.
 *
 * Connects to `ws(s)://<GAMING_API>/gaming/boss/stream/<sessionId>` as soon
 * as `sessionId` becomes non-null.  The socket is torn down when the
 * component unmounts or when `sessionId` changes to a different value.
 *
 * Malformed or unparseable JSON messages are silently ignored — the last
 * valid `battleState` is preserved.
 *
 * @param sessionId - Battle session ID returned by `POST /gaming/boss/start`,
 *   or `null` to keep the socket closed.
 * @returns `{ battleState, sendAttack, isConnected }`
 *
 * @example
 * ```tsx
 * const { battleState, sendAttack, isConnected } = useBattleStream(sessionId)
 *
 * // Render HP bars from battleState
 * // Call sendAttack({ sessionId, answerCorrect, baseDamage }) when player attacks
 * ```
 */
export function useBattleStream(sessionId: string | null): UseBattleStreamResult {
  const [battleState, setBattleState] = useState<BattleState | null>(null)
  const [isConnected, setIsConnected] = useState(false)

  // Keep a stable ref to the live socket so sendAttack can read it without
  // needing to be recreated every time sessionId or connection state changes.
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    if (!sessionId) return

    const wsUrl = `${toWsUrl(GAMING_API)}/gaming/boss/stream/${sessionId}`
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      setIsConnected(true)
    }

    ws.onmessage = (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data as string) as BattleEvent
        setBattleState({
          bossHp: data.boss_hp,
          playerHp: data.player_hp,
          damageDealt: data.damage_dealt,
          comboCount: data.combo_count,
          phase: data.phase,
        })
      } catch {
        // Non-fatal — malformed messages are swallowed; last valid state persists.
      }
    }

    ws.onclose = () => {
      setIsConnected(false)
    }

    ws.onerror = () => {
      setIsConnected(false)
    }

    return () => {
      wsRef.current = null
      ws.close()
    }
  }, [sessionId])

  const sendAttack = useCallback((payload: SendAttackPayload) => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    ws.send(
      JSON.stringify({
        type: 'attack',
        session_id: payload.sessionId,
        answer_correct: payload.answerCorrect,
        base_damage: payload.baseDamage,
      }),
    )
  }, [])

  return { battleState, sendAttack, isConnected }
}
