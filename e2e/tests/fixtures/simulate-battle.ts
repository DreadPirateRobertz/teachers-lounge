import * as ws from 'ws'

const BASE_URL = process.env.BASE_URL || 'http://localhost:3000'
// Derive WS URL from BASE_URL (http → ws, https → wss)
const WS_URL = BASE_URL.replace(/^http/, 'ws')

/**
 * Result of a simulated boss battle.
 */
export interface BattleResult {
  /** Whether the battle ended in a victory (boss HP reached 0). */
  victory: boolean
  /** Final boss HP reported by the last event. */
  finalBossHp: number
  /** Number of damage events received. */
  damageEvents: number
  /** Error message if the WS connection failed. */
  error?: string
}

/**
 * Simulate a full boss battle via WebSocket by sending damage events until
 * the boss HP reaches 0 (or a timeout is hit).
 *
 * The gaming service broadcasts HP-update events over WS; this client sends
 * simulated attack events and tracks the boss HP.
 *
 * @param token    - Bearer token for the authenticated user.
 * @param bossId   - Numeric boss ID (default: 1 = The Atom).
 * @param timeoutMs - Max wait before giving up (default: 30 s).
 */
export function simulateBattle(
  token: string,
  bossId: number = 1,
  timeoutMs: number = 30_000,
): Promise<BattleResult> {
  return new Promise((resolve) => {
    const wsUrl = `${WS_URL}/api/gaming/boss-battle/${bossId}/ws?token=${encodeURIComponent(token)}`
    let socket: ws.WebSocket | null = null
    let finalBossHp = 100
    let damageEvents = 0
    let resolved = false

    const done = (result: BattleResult) => {
      if (resolved) return
      resolved = true
      try {
        socket?.close()
      } catch {
        // ignore
      }
      clearTimeout(timer)
      resolve(result)
    }

    const timer = setTimeout(() => {
      done({ victory: false, finalBossHp, damageEvents, error: 'timeout' })
    }, timeoutMs)

    try {
      socket = new ws.WebSocket(wsUrl, {
        headers: { Authorization: `Bearer ${token}` },
      })
    } catch (err) {
      done({ victory: false, finalBossHp, damageEvents, error: String(err) })
      return
    }

    socket.on('error', (err) => {
      done({ victory: false, finalBossHp, damageEvents, error: err.message })
    })

    socket.on('open', () => {
      // Send a sequence of attack events — each deals 10% boss HP damage
      const sendAttack = () => {
        if (resolved) return
        socket?.send(JSON.stringify({ type: 'attack', damage: 10 }))
      }
      // Spread attacks over 20 s so the backend can process them
      for (let i = 0; i < 12; i++) {
        setTimeout(sendAttack, i * 1_500)
      }
    })

    socket.on('message', (data) => {
      try {
        const msg = JSON.parse(data.toString()) as Record<string, unknown>
        if (typeof msg.boss_hp === 'number') {
          finalBossHp = msg.boss_hp
          damageEvents++
          if (finalBossHp <= 0) {
            done({ victory: true, finalBossHp: 0, damageEvents })
          }
        }
        if (msg.type === 'battle_end') {
          done({ victory: msg.outcome === 'victory', finalBossHp, damageEvents })
        }
      } catch {
        // ignore non-JSON frames
      }
    })

    socket.on('close', () => {
      // If not already resolved, treat close as end of battle
      done({ victory: finalBossHp <= 0, finalBossHp, damageEvents })
    })
  })
}
