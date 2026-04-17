'use client'

/**
 * BossBattleClient.tsx
 *
 * Client-side state machine for a boss battle.
 *
 * Phase flow:
 *   intro → question → attack → resolve → question (loop)
 *                                       → victory | defeat
 *
 * - intro:   Boss introduction screen; player clicks "Begin Battle".
 * - question: A multiple-choice question from the quiz API is shown alongside
 *             the BossScene and health-bar HUD. Power-ups can be activated here.
 * - attack:  The player's chosen answer triggers attack animations on BossScene
 *            (1 s). Both the quiz-answer and battle-attack APIs are called.
 * - resolve: Damage numbers and answer explanation are shown briefly (~2.5 s),
 *            then the machine advances to the next question or ends the battle.
 * - victory / defeat: End-screen sub-components.
 *
 * This component owns all interactive state. The page server component fetches
 * the boss definition and passes it down.
 */

import dynamic from 'next/dynamic'
import Link from 'next/link'
import { useCallback, useEffect, useRef, useState } from 'react'
import { type AnimationState, type BossVisualDef, getRandomTaunt } from './BossCharacterLibrary'
import BossHUD, { type PowerUp } from './BossHUD'
import BattleResolve from './BattleResolve'
import LootReveal, { type LootItem } from './LootReveal'
import QuestionCard, { type BattleQuestion } from './QuestionCard'
import ErrorBoundary from '@/components/ErrorBoundary'
import useSwipeGesture, { SwipeDirection } from '@/hooks/useSwipeGesture'
import { useBattleStream } from '@/hooks/useBattleStream'

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

// ─── API response types ───────────────────────────────────────────────────────

interface StartBattleResponse {
  session: {
    session_id: string
    boss_hp: number
    boss_max_hp: number
    player_hp: number
    player_max_hp: number
  }
}

interface StartQuizResponse {
  session: { id: string }
  question: BattleQuestion | null
}

interface SubmitAnswerResponse {
  correct: boolean
  correct_key: string
  explanation: string
  next_question: BattleQuestion | null
}

interface AttackResponse {
  player_damage_dealt: number
  boss_damage_dealt: number
  boss_hp: number
  player_hp: number
  phase: 'active' | 'victory' | 'defeat'
  turn: number
}

// ─── State types ──────────────────────────────────────────────────────────────

/** The five phases of a boss battle. */
type BattlePhase = 'intro' | 'question' | 'attack' | 'resolve' | 'victory' | 'defeat'

/** Data shown on the resolve screen after each round. */
interface ResolveData {
  playerDamage: number
  bossDamage: number
  correct: boolean
  explanation: string
}

// ─── Component props ──────────────────────────────────────────────────────────

/** Props passed from the server-side page. */
interface BossBattleClientProps {
  /** Full visual definition for this boss (from BossCharacterLibrary). */
  boss: BossVisualDef
  /** Authenticated user ID, for API calls. */
  userId: string
  /** Starting gem count from the user's profile. */
  initialGems: number
}

// ─── Component ────────────────────────────────────────────────────────────────

/**
 * BossBattleClient manages the full interactive boss battle experience,
 * driving the phase state machine, quiz questions, animations, and all
 * player interactions.
 */
export default function BossBattleClient({ boss, userId, initialGems }: BossBattleClientProps) {
  // ── Battle session ─────────────────────────────────────────────────────────
  const [phase, setPhase] = useState<BattlePhase>('intro')
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [bossHP, setBossHP] = useState(0)
  const [bossMaxHP, setBossMaxHP] = useState(0)
  const [playerHP, setPlayerHP] = useState(0)
  const [playerMaxHP, setPlayerMaxHP] = useState(0)
  const [turn, setTurn] = useState(0)
  const [gems, setGems] = useState(initialGems)
  const [taunt, setTaunt] = useState<string | null>(null)
  const [combo, setCombo] = useState(0)

  // ── Real-time battle stream ────────────────────────────────────────────────
  // Subscribed once a battle session exists. The REST attack flow remains the
  // canonical action path (it returns explanation + correct_key for the resolve
  // overlay), but the WS push lets us reflect server-authoritative HP / combo /
  // phase changes the moment they happen — including ones triggered outside
  // this client (e.g. boss-attack server ticks).
  const { battleState: streamState } = useBattleStream(sessionId)

  // ── Quiz session ───────────────────────────────────────────────────────────
  const [quizSessionId, setQuizSessionId] = useState<string | null>(null)
  const [currentQuestion, setCurrentQuestion] = useState<BattleQuestion | null>(null)
  const [chosenKey, setChosenKey] = useState<string | null>(null)
  const [correctKey, setCorrectKey] = useState<string | null>(null)

  // ── Visual state ───────────────────────────────────────────────────────────
  const [animState, setAnimState] = useState<AnimationState>('idle')
  const [resolveData, setResolveData] = useState<ResolveData | null>(null)

  // ── Shared ─────────────────────────────────────────────────────────────────
  const [error, setError] = useState<string | null>(null)
  const [actionPending, setActionPending] = useState(false)

  // Prevent double-firing resolve → question transition.
  const resolveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // ── Start battle + quiz ────────────────────────────────────────────────────

  const startBattle = useCallback(async () => {
    setError(null)
    setActionPending(true)
    try {
      // Kick off both sessions in parallel.
      const [battleResp, quizResp] = await Promise.all([
        apiFetch<StartBattleResponse>('/gaming/boss/start', {
          method: 'POST',
          body: JSON.stringify({ user_id: userId, boss_id: boss.id }),
        }),
        apiFetch<StartQuizResponse>('/gaming/quiz/start', {
          method: 'POST',
          body: JSON.stringify({ user_id: userId, topic: boss.topic, question_count: 20 }),
        }),
      ])

      setSessionId(battleResp.session.session_id)
      setBossHP(battleResp.session.boss_hp)
      setBossMaxHP(battleResp.session.boss_max_hp)
      setPlayerHP(battleResp.session.player_hp)
      setPlayerMaxHP(battleResp.session.player_max_hp)
      setTurn(1)

      setQuizSessionId(quizResp.session.id)
      setCurrentQuestion(quizResp.question)

      setChosenKey(null)
      setCorrectKey(null)
      setPhase('question')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to start battle')
    } finally {
      setActionPending(false)
    }
  }, [userId, boss.id, boss.topic])

  // ── Player selects an answer ───────────────────────────────────────────────

  const submitAnswer = useCallback(
    async (key: string) => {
      if (!sessionId || !quizSessionId || !currentQuestion) return
      if (phase !== 'question' || chosenKey !== null || actionPending) return

      setChosenKey(key)
      setPhase('attack')
      setActionPending(true)

      // Trigger boss attack animation for wrong answers, damage flash for correct.
      setAnimState(key === 'pending_check' ? 'idle' : 'attack')

      try {
        const difficulty = currentQuestion.difficulty ?? 1
        const baseDamage = difficulty * 10

        // Step 1: Submit quiz answer to find out if it was correct.
        // The battle attack must be sequential (it needs the correct flag).
        const quizResp = await apiFetch<SubmitAnswerResponse>(
          `/gaming/quiz/sessions/${quizSessionId}/answer`,
          {
            method: 'POST',
            body: JSON.stringify({
              user_id: userId,
              question_id: currentQuestion.id,
              chosen_key: key,
            }),
          },
        )

        const correct = quizResp.correct
        setCorrectKey(quizResp.correct_key)

        // Trigger appropriate BossScene animation.
        if (correct) {
          setAnimState('damage')
          setTimeout(() => setAnimState('idle'), 800)
          setTaunt(null)
        } else {
          setAnimState('attack')
          setTimeout(() => setAnimState('idle'), 1200)
          setTaunt(getRandomTaunt(boss.id))
        }

        // Now submit battle attack with the verified correct flag.
        const battleResp = await apiFetch<AttackResponse>('/gaming/boss/attack', {
          method: 'POST',
          body: JSON.stringify({
            session_id: sessionId,
            answer_correct: correct,
            base_damage: baseDamage,
          }),
        })

        setBossHP(battleResp.boss_hp)
        setPlayerHP(battleResp.player_hp)
        setTurn(battleResp.turn + 1)

        setResolveData({
          playerDamage: battleResp.player_damage_dealt,
          bossDamage: battleResp.boss_damage_dealt,
          correct,
          explanation: quizResp.explanation ?? '',
        })
        setPhase('resolve')

        // Queue next question (or end of quiz — fall back to re-using topic).
        const nextQ = quizResp.next_question ?? null

        if (battleResp.phase === 'victory') {
          setAnimState('death')
          // handleDeathComplete will transition to victory phase.
        } else if (battleResp.phase === 'defeat') {
          // Brief resolve display, then defeat.
          resolveTimerRef.current = setTimeout(() => {
            setPhase('defeat')
          }, 2500)
        } else {
          // Advance to next question after resolve display.
          resolveTimerRef.current = setTimeout(() => {
            setChosenKey(null)
            setCorrectKey(null)
            setResolveData(null)
            if (nextQ) {
              setCurrentQuestion(nextQ)
            }
            // If quiz ran out of questions, keep current question recycled
            // (backend session expired — start a fresh quiz on next turn).
            setPhase('question')
          }, 2500)
        }
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Attack failed')
        setPhase('question')
        setChosenKey(null)
        setAnimState('idle')
      } finally {
        setActionPending(false)
      }
    },
    [sessionId, quizSessionId, currentQuestion, phase, chosenKey, actionPending, userId, boss.id],
  )

  // ── Boss death animation complete → victory ────────────────────────────────

  const handleDeathComplete = useCallback(() => {
    setPhase('victory')
  }, [])

  // ── Activate a power-up ───────────────────────────────────────────────────

  const activatePowerUp = useCallback(
    async (type: PowerUp['type']) => {
      if (!sessionId || phase !== 'question') return
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
      // Ignore forfeit errors — still transition to defeat.
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

  // ── Cleanup pending timers on unmount ─────────────────────────────────────

  useEffect(() => {
    return () => {
      if (resolveTimerRef.current) clearTimeout(resolveTimerRef.current)
    }
  }, [])

  // ── Mirror real-time stream state into local state ────────────────────────
  // The WS hook delivers normalised camelCase snapshots; we apply them to HP /
  // combo so the HUD reflects server truth as soon as it arrives. Phase
  // transitions (victory / defeat) from the stream nudge the state machine
  // when the REST attack response hasn't done so yet — defensive in case the
  // attack POST is slower than the broadcast event.
  useEffect(() => {
    if (!streamState) return
    setBossHP(streamState.bossHp)
    setPlayerHP(streamState.playerHp)
    setCombo(streamState.comboCount)
    if (streamState.phase === 'victory' && phase !== 'victory' && animState !== 'death') {
      setAnimState('death')
    } else if (streamState.phase === 'defeat' && phase !== 'defeat') {
      setPhase('defeat')
    }
  }, [streamState, phase, animState])

  // ── Swipe gesture: right = option A, left = last option ──────────────────

  const {
    onTouchStart,
    onTouchEnd,
    swipeDirection,
    reset: resetSwipe,
  } = useSwipeGesture({
    onSwipe: (dir) => {
      if (actionPending || phase !== 'question' || !currentQuestion) return
      const opts = currentQuestion.options
      const key = dir === SwipeDirection.Right ? opts[0]?.key : opts[opts.length - 1]?.key
      if (key) submitAnswer(key)
    },
  })

  // ── Determine whether active battle is displayed ──────────────────────────

  const showBattle =
    phase === 'question' || phase === 'attack' || phase === 'resolve' || animState === 'death'

  // ─── Render ───────────────────────────────────────────────────────────────

  return (
    <div className="flex flex-col items-center gap-6 min-h-screen bg-bg-deep py-8 px-4">
      {/* ── Intro screen ── */}
      {phase === 'intro' && (
        <StartScreen boss={boss} onStart={startBattle} loading={actionPending} />
      )}

      {/* ── Active battle (question / attack / resolve) ── */}
      {showBattle && (
        <div
          className="flex flex-col items-center gap-5 w-full max-w-md"
          style={{ touchAction: 'pan-y' }}
          onTouchStart={onTouchStart as React.TouchEventHandler}
          onTouchEnd={(e) => {
            ;(onTouchEnd as React.TouchEventHandler)(e)
            setTimeout(resetSwipe, 600)
          }}
        >
          {/* Boss 3-D scene */}
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

          {/* Health bars + power-up HUD */}
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
            disabled={actionPending || phase !== 'question'}
            comboCount={combo}
          />

          {/* Question card — visible during question phase */}
          {(phase === 'question' || phase === 'attack') && currentQuestion && (
            <QuestionCard
              question={currentQuestion}
              chosenKey={chosenKey}
              correctKey={correctKey}
              onAnswer={submitAnswer}
              disabled={actionPending || phase !== 'question'}
            />
          )}

          {/* Resolve overlay — visible after attack */}
          {phase === 'resolve' && resolveData && (
            <BattleResolve
              playerDamage={resolveData.playerDamage}
              bossDamage={resolveData.bossDamage}
              correct={resolveData.correct}
              explanation={resolveData.explanation}
            />
          )}

          {/* Swipe hint */}
          {swipeDirection && phase === 'question' && (
            <p aria-live="polite" className="text-xs font-mono text-text-dim animate-fade-in">
              {swipeDirection === SwipeDirection.Right ? '→ First option' : '← Last option'}
            </p>
          )}

          {/* Forfeit button — only visible during question phase */}
          {phase === 'question' && (
            <button
              onClick={forfeit}
              disabled={actionPending}
              className="min-h-[44px] px-4 py-3 rounded-lg text-xs font-mono text-text-dim
                border border-border-dim hover:border-neon-pink/30 transition-colors
                disabled:opacity-40 disabled:cursor-not-allowed touch-manipulation"
              style={{ touchAction: 'manipulation' }}
            >
              Forfeit
            </button>
          )}

          {error && <p className="text-xs text-neon-pink font-mono">{error}</p>}
        </div>
      )}

      {/* ── Victory ── */}
      {phase === 'victory' && <VictoryScreen boss={boss} turns={turn} />}

      {/* ── Defeat ── */}
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

/**
 * Derive client-side victory rewards from the battle outcome.
 *
 * Kept pure + exported for unit testing. The backend is the source of truth
 * for persisted rewards; this is a presentational approximation for the
 * post-battle reveal. Values are deterministic given `turns` and `boss`.
 */
export function computeVictoryLoot(boss: BossVisualDef, turns: number): LootItem[] {
  const xp = 100 + turns * 10
  const gems = 25
  return [
    { key: 'xp', icon: '⚡', label: 'XP Earned', amount: xp },
    { key: 'gems', icon: '💎', label: 'Gems', amount: gems },
    { key: 'badge', icon: '🏅', label: `Badge: ${boss.name} Slayer`, amount: null },
  ]
}

function VictoryScreen({ boss, turns }: { boss: BossVisualDef; turns: number }) {
  const loot = computeVictoryLoot(boss, turns)
  return (
    <div className="flex flex-col items-center gap-4 text-center">
      <div className="text-4xl">🏆</div>
      <h2 className="text-2xl font-mono font-bold text-neon-gold text-glow-gold">VICTORY!</h2>
      <p className="text-sm text-text-base font-mono">
        You defeated <span style={{ color: boss.primaryColor }}>{boss.name}</span>
      </p>
      <LootReveal items={loot} />
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
