'use client'

/**
 * BossBattleClient.tsx
 *
 * Client-side state machine for a boss battle. Connects to the gaming-service
 * battle API (start, attack, powerup, forfeit) and drives the Three.js
 * BossScene animation state and BossHUD display.
 *
 * The page server component fetches the boss definition and passes it down.
 * This component owns all interactive state.
 */

import dynamic from 'next/dynamic'
import Link from 'next/link'
import { useCallback, useEffect, useState } from 'react'
import { type AnimationState, type BossVisualDef, getRandomTaunt } from './BossCharacterLibrary'
import BossHUD, { type PowerUp } from './BossHUD'
import ErrorBoundary from '@/components/ErrorBoundary'
import useSwipeGesture, { SwipeDirection } from '@/hooks/useSwipeGesture'

// BossScene is client-only WebGL; dynamic import prevents SSR errors.
const BossScene = dynamic(() => import('./BossScene'), { ssr: false })

// ─── API helpers ──────────────────────────────────────────────────────────────

const GAMING_API = process.env.NEXT_PUBLIC_GAMING_SERVICE_URL ?? 'http://localhost:8083'

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${GAMING_API}${path}`, {
    ...init,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
  })
  if (!res.ok) {
    const body = await res.text().catch(() => '')
    throw new Error(`${res.status}: ${body}`)
  }
  return res.json() as Promise<T>
}

interface StartBattleResponse {
  session: {
    session_id: string
    boss_hp: number
    boss_max_hp: number
    player_hp: number
    player_max_hp: number
  }
}

interface AttackResponse {
  player_damage_dealt: number
  boss_damage_dealt: number
  boss_hp: number
  player_hp: number
  phase: 'active' | 'victory' | 'defeat'
  turn: number
}

// ─── Component ────────────────────────────────────────────────────────────────

/** Props passed from the server-side page. */
interface BossBattleClientProps {
  /** Full visual definition for this boss (from BossCharacterLibrary). */
  boss: BossVisualDef
  /** Authenticated user ID, for API calls. */
  userId: string
  /** Starting gem count from the user's profile. */
  initialGems: number
}

type BattlePhase = 'start' | 'active' | 'victory' | 'defeat'

/**
 * BossBattleClient manages the full interactive boss battle experience.
 * It owns the battle session, drives animations, and wires player inputs.
 */
export default function BossBattleClient({ boss, userId, initialGems }: BossBattleClientProps) {
  const [phase, setPhase] = useState<BattlePhase>('start')
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [bossHP, setBossHP] = useState(0)
  const [bossMaxHP, setBossMaxHP] = useState(0)
  const [playerHP, setPlayerHP] = useState(0)
  const [playerMaxHP, setPlayerMaxHP] = useState(0)
  const [turn, setTurn] = useState(0)
  const [gems, setGems] = useState(initialGems)
  const [taunt, setTaunt] = useState<string | null>(null)
  const [animState, setAnimState] = useState<AnimationState>('idle')
  const [error, setError] = useState<string | null>(null)
  const [actionPending, setActionPending] = useState(false)

  // ── Start the battle ──────────────────────────────────────────────────────

  const startBattle = useCallback(async () => {
    setError(null)
    setActionPending(true)
    try {
      const resp = await apiFetch<StartBattleResponse>('/gaming/boss/start', {
        method: 'POST',
        body: JSON.stringify({ user_id: userId, boss_id: boss.id }),
      })
      setSessionId(resp.session.session_id)
      setBossHP(resp.session.boss_hp)
      setBossMaxHP(resp.session.boss_max_hp)
      setPlayerHP(resp.session.player_hp)
      setPlayerMaxHP(resp.session.player_max_hp)
      setPhase('active')
      setTurn(1)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to start battle')
    } finally {
      setActionPending(false)
    }
  }, [userId, boss.id])

  // ── Submit an answer (correct or wrong) ───────────────────────────────────

  const submitAnswer = useCallback(
    async (correct: boolean) => {
      if (!sessionId || phase !== 'active') return
      setActionPending(true)
      try {
        // Trigger boss attack animation if wrong answer
        if (!correct) {
          setAnimState('attack')
          setTimeout(() => setAnimState('idle'), 1200)
          setTaunt(getRandomTaunt(boss.id))
        } else {
          setAnimState('damage')
          setTimeout(() => setAnimState('idle'), 500)
          setTaunt(null)
        }

        const resp = await apiFetch<AttackResponse>('/gaming/boss/attack', {
          method: 'POST',
          body: JSON.stringify({
            session_id: sessionId,
            answer_correct: correct,
            base_damage: 40,
          }),
        })

        setBossHP(resp.boss_hp)
        setPlayerHP(resp.player_hp)
        setTurn(resp.turn + 1)

        if (resp.phase === 'victory') {
          setAnimState('death')
          // Wait for death animation to complete before switching phase
        } else if (resp.phase === 'defeat') {
          setPhase('defeat')
        }
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Attack failed')
      } finally {
        setActionPending(false)
      }
    },
    [sessionId, phase, boss.id],
  )

  // ── Activate a power-up ───────────────────────────────────────────────────

  const activatePowerUp = useCallback(
    async (type: PowerUp['type']) => {
      if (!sessionId || phase !== 'active') return
      setActionPending(true)
      try {
        const resp = await apiFetch<{ gems_left: number }>('/gaming/boss/powerup', {
          method: 'POST',
          body: JSON.stringify({ session_id: sessionId, power_up: type }),
        })
        setGems(resp.gems_left)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Power-up failed')
      } finally {
        setActionPending(false)
      }
    },
    [sessionId, phase],
  )

  // ── Death animation complete ──────────────────────────────────────────────

  const handleDeathComplete = useCallback(() => {
    setPhase('victory')
  }, [])

  // ── Forfeit ───────────────────────────────────────────────────────────────

  const forfeit = useCallback(async () => {
    if (!sessionId) return
    setActionPending(true)
    try {
      await apiFetch('/gaming/boss/forfeit', {
        method: 'POST',
        body: JSON.stringify({ session_id: sessionId }),
      })
    } catch {
      // Ignore forfeit errors — still transition to defeat
    } finally {
      setActionPending(false)
      setPhase('defeat')
    }
  }, [sessionId])

  // ── Clear taunt after a delay ─────────────────────────────────────────────

  useEffect(() => {
    if (!taunt) return
    const t = setTimeout(() => setTaunt(null), 3000)
    return () => clearTimeout(t)
  }, [taunt])

  // ── Swipe gesture: right = correct, left = wrong ─────────────────────────

  const { onTouchStart, onTouchEnd, swipeDirection, reset: resetSwipe } = useSwipeGesture({
    onSwipe: (dir) => {
      if (actionPending || phase !== 'active') return
      submitAnswer(dir === SwipeDirection.Right)
    },
  })

  // ─── Render ───────────────────────────────────────────────────────────────

  return (
    <div className="flex flex-col items-center gap-6 min-h-screen bg-bg-deep py-8 px-4">
      {/* Start screen */}
      {phase === 'start' && (
        <StartScreen boss={boss} onStart={startBattle} loading={actionPending} />
      )}

      {/* Active battle */}
      {(phase === 'active' || animState === 'death') && (
        /*
         * Swipe zone: right = correct, left = wrong.
         * `touch-action: pan-y` allows vertical scroll while intercepting
         * horizontal swipes.  A hint banner fades in briefly after a swipe
         * to confirm the gesture was received.
         */
        <div
          className="flex flex-col items-center gap-6 w-full max-w-md"
          style={{ touchAction: 'pan-y' }}
          onTouchStart={onTouchStart as React.TouchEventHandler}
          onTouchEnd={(e) => {
            ;(onTouchEnd as React.TouchEventHandler)(e)
            setTimeout(resetSwipe, 600)
          }}
        >
          <ErrorBoundary
            componentName="BossScene"
            fallback={
              <div className="flex flex-col items-center justify-center w-[360px] h-[320px] rounded-lg bg-bg-card border border-neon-pink/30 gap-3">
                <span className="text-3xl">💥</span>
                <p className="text-xs text-text-dim font-mono">WebGL failed to initialise.</p>
                <p className="text-[10px] text-text-dim">Try refreshing the page.</p>
              </div>
            }
          >
            <BossScene
              boss={boss}
              animationState={animState}
              onDeathComplete={handleDeathComplete}
              width={360}
              height={320}
            />
          </ErrorBoundary>
          <BossHUD
            boss={boss}
            bossHP={bossHP}
            bossMaxHP={bossMaxHP}
            playerHP={playerHP}
            playerMaxHP={playerMaxHP}
            turn={turn}
            gems={gems}
            taunt={taunt}
            onPowerUpAction={activatePowerUp}
            disabled={actionPending}
          />
          {/* Swipe hint — briefly shown after a horizontal swipe on mobile */}
          {swipeDirection && (
            <p
              aria-live="polite"
              className="text-xs font-mono text-text-dim animate-fade-in"
            >
              {swipeDirection === SwipeDirection.Right ? '→ Correct' : '← Wrong'}
            </p>
          )}

          {/* Demo attack buttons — replace with real quiz UI in Phase 4.
              Min touch target 44×44 px (Apple HIG) for K-12 mobile usability. */}
          <div className="flex gap-3 w-full">
            <button
              onClick={() => submitAnswer(true)}
              disabled={actionPending}
              className="flex-1 min-h-[44px] px-5 py-3 rounded-lg text-sm font-mono font-bold
                bg-neon-green/10 border border-neon-green/40 text-neon-green
                hover:bg-neon-green/20 active:bg-neon-green/30 transition-colors
                disabled:opacity-40 disabled:cursor-not-allowed
                touch-manipulation"
              style={{ touchAction: 'manipulation' }}
            >
              ✓ Correct
            </button>
            <button
              onClick={() => submitAnswer(false)}
              disabled={actionPending}
              className="flex-1 min-h-[44px] px-5 py-3 rounded-lg text-sm font-mono font-bold
                bg-neon-pink/10 border border-neon-pink/40 text-neon-pink
                hover:bg-neon-pink/20 active:bg-neon-pink/30 transition-colors
                disabled:opacity-40 disabled:cursor-not-allowed
                touch-manipulation"
              style={{ touchAction: 'manipulation' }}
            >
              ✗ Wrong
            </button>
            <button
              onClick={forfeit}
              disabled={actionPending}
              className="min-h-[44px] px-4 py-3 rounded-lg text-xs font-mono text-text-dim
                border border-border-dim hover:border-neon-pink/30 transition-colors
                disabled:opacity-40 disabled:cursor-not-allowed
                touch-manipulation"
              style={{ touchAction: 'manipulation' }}
            >
              Forfeit
            </button>
          </div>
          {error && <p className="text-xs text-neon-pink font-mono">{error}</p>}
        </div>
      )}

      {/* Victory */}
      {phase === 'victory' && <VictoryScreen boss={boss} />}

      {/* Defeat */}
      {phase === 'defeat' && <DefeatScreen boss={boss} onRetry={startBattle} />}
    </div>
  )
}

// ─── Sub-screens ─────────────────────────────────────────────────────────────

function StartScreen({
  boss,
  onStart,
  loading,
}: {
  boss: BossVisualDef
  onStart: () => void
  loading: boolean
}) {
  return (
    <div className="flex flex-col items-center gap-6 max-w-sm text-center">
      <div
        className="text-6xl font-mono font-black tracking-widest"
        style={{ color: boss.primaryColor, textShadow: `0 0 20px ${boss.primaryColor}` }}
      >
        {boss.name}
      </div>
      <p className="text-sm text-text-base font-mono">
        Topic: <span className="text-neon-blue">{boss.topic.replace(/_/g, ' ')}</span>
      </p>
      <p className="text-xs text-text-dim leading-relaxed italic">
        &ldquo;{boss.tauntPool[0]}&rdquo;
      </p>
      <button
        onClick={onStart}
        disabled={loading}
        className="px-8 py-3 rounded-xl font-mono font-bold text-sm border transition-all
          disabled:opacity-50 disabled:cursor-not-allowed active:scale-95"
        style={{
          color: boss.primaryColor,
          borderColor: `${boss.primaryColor}55`,
          backgroundColor: `${boss.primaryColor}0f`,
          boxShadow: `0 0 12px ${boss.primaryColor}33`,
        }}
      >
        {loading ? 'Starting…' : '⚔️ Begin Battle'}
      </button>
    </div>
  )
}

function VictoryScreen({ boss }: { boss: BossVisualDef }) {
  return (
    <div className="flex flex-col items-center gap-4 text-center">
      <div className="text-4xl">🏆</div>
      <h2 className="text-2xl font-mono font-bold text-neon-gold text-glow-gold">VICTORY!</h2>
      <p className="text-sm text-text-base font-mono">
        You defeated <span style={{ color: boss.primaryColor }}>{boss.name}</span>
      </p>
      <Link
        href="/"
        className="mt-4 text-sm text-neon-blue border border-neon-blue/30 px-5 py-2 rounded-lg
          hover:bg-neon-blue/10 transition-colors font-mono"
      >
        ← Back to Tutor
      </Link>
    </div>
  )
}

function DefeatScreen({ boss, onRetry }: { boss: BossVisualDef; onRetry: () => void }) {
  return (
    <div className="flex flex-col items-center gap-4 text-center">
      <div className="text-4xl">💀</div>
      <h2 className="text-2xl font-mono font-bold text-neon-pink">DEFEATED</h2>
      <p className="text-sm text-text-dim font-mono italic">&ldquo;{boss.tauntPool[0]}&rdquo;</p>
      <div className="flex gap-3 mt-4">
        <button
          onClick={onRetry}
          className="text-sm text-neon-blue border border-neon-blue/30 px-5 py-2 rounded-lg
            hover:bg-neon-blue/10 transition-colors font-mono"
        >
          Try Again
        </button>
        <Link
          href="/"
          className="text-sm text-text-dim border border-border-dim px-5 py-2 rounded-lg
            hover:border-neon-pink/30 transition-colors font-mono"
        >
          ← Back to Tutor
        </Link>
      </div>
    </div>
  )
}
