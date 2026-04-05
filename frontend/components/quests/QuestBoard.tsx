'use client'

import { useEffect, useState, useCallback } from 'react'

// ── Types ────────────────────────────────────────────────────────────────────

interface QuestState {
  id: string
  title: string
  description: string
  progress: number
  target: number
  completed: boolean
  xp_reward: number
  gems_reward: number
}

interface DailyQuestsResponse {
  quests: QuestState[]
}

// ── Quest icons keyed by ID ───────────────────────────────────────────────────

const QUEST_ICON: Record<string, string> = {
  questions_answered: '🔍',
  keep_streak_alive: '🔥',
  master_new_concept: '⭐',
}

// ── Reset timer ───────────────────────────────────────────────────────────────

function useResetCountdown() {
  const [label, setLabel] = useState('')

  useEffect(() => {
    function calc() {
      const now = new Date()
      const next = new Date(now)
      next.setUTCHours(24, 0, 0, 0)
      const diffMs = next.getTime() - now.getTime()
      const h = Math.floor(diffMs / 3_600_000)
      const m = Math.floor((diffMs % 3_600_000) / 60_000)
      const s = Math.floor((diffMs % 60_000) / 1_000)
      setLabel(
        `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`,
      )
    }
    calc()
    const id = setInterval(calc, 1_000)
    return () => clearInterval(id)
  }, [])

  return label
}

// ── Main component ────────────────────────────────────────────────────────────

export default function QuestBoard() {
  const [quests, setQuests] = useState<QuestState[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const resetIn = useResetCountdown()

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/gaming/quests', { cache: 'no-store' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data: DailyQuestsResponse = await res.json()
      setQuests(data.quests ?? [])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load quests')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const allComplete = quests.length > 0 && quests.every((q) => q.completed)
  const totalXP = quests.reduce((s, q) => s + q.xp_reward, 0)
  const totalGems = quests.reduce((s, q) => s + q.gems_reward, 0)
  const earnedXP = quests.filter((q) => q.completed).reduce((s, q) => s + q.xp_reward, 0)
  const earnedGems = quests.filter((q) => q.completed).reduce((s, q) => s + q.gems_reward, 0)

  return (
    <div className="flex flex-col gap-4">
      {/* Header row */}
      <div className="flex items-start justify-between">
        <div>
          <h2 className="font-mono text-base font-bold text-text-bright tracking-wide">
            Daily Quests
          </h2>
          <div className="text-xs text-text-dim mt-0.5">Complete quests to earn XP and gems</div>
        </div>
        <div className="text-right">
          <div className="text-[10px] text-text-dim uppercase tracking-wider">Resets in</div>
          <div className="font-mono text-sm font-bold text-neon-blue text-glow-blue">{resetIn}</div>
        </div>
      </div>

      {/* Reward summary bar */}
      <div className="flex items-center gap-3 bg-bg-card border border-border-dim rounded-lg px-3 py-2">
        <span className="text-xs text-text-dim">Today&apos;s haul:</span>
        <span className="font-mono text-xs text-neon-blue text-glow-blue">
          +{earnedXP}/{totalXP} XP
        </span>
        <span className="text-border-mid">·</span>
        <span className="font-mono text-xs text-neon-pink">
          {earnedGems}/{totalGems} 💎
        </span>
        {allComplete && (
          <span className="ml-auto text-xs font-semibold text-neon-green text-glow-green animate-pulse-slow">
            ✓ All done!
          </span>
        )}
      </div>

      {/* Quest cards */}
      {loading && (
        <div className="flex flex-col gap-3">
          {[0, 1, 2].map((i) => (
            <QuestCardSkeleton key={i} />
          ))}
        </div>
      )}

      {error && (
        <div className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg px-3 py-2">
          {error}
        </div>
      )}

      {!loading && !error && (
        <div className="flex flex-col gap-3">
          {quests.map((quest) => (
            <QuestCard key={quest.id} quest={quest} />
          ))}
        </div>
      )}
    </div>
  )
}

// ── Quest card ────────────────────────────────────────────────────────────────

function QuestCard({ quest }: { quest: QuestState }) {
  const pct =
    quest.target > 0 ? Math.min(100, Math.round((quest.progress / quest.target) * 100)) : 0
  const icon = QUEST_ICON[quest.id] ?? '📋'

  return (
    <div
      className={`rounded-lg border p-3 transition-all ${
        quest.completed
          ? 'bg-neon-green/5 border-neon-green/25'
          : 'bg-bg-card border-border-mid hover:border-border-mid/80'
      }`}
    >
      {/* Top row: icon + title + rewards */}
      <div className="flex items-start gap-2.5">
        <div
          className={`text-2xl leading-none flex-shrink-0 mt-0.5 ${
            quest.completed ? 'opacity-60' : ''
          }`}
        >
          {icon}
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between gap-2">
            <span
              className={`text-sm font-semibold leading-tight ${
                quest.completed ? 'text-text-dim line-through' : 'text-text-bright'
              }`}
            >
              {quest.title}
            </span>

            {/* Rewards */}
            <div className="flex items-center gap-1.5 flex-shrink-0">
              <span className="font-mono text-[11px] text-neon-blue">+{quest.xp_reward} XP</span>
              <span className="text-border-mid text-[10px]">·</span>
              <span className="font-mono text-[11px] text-neon-pink">+{quest.gems_reward}💎</span>
            </div>
          </div>

          <div className="text-[11px] text-text-dim mt-0.5 leading-tight">{quest.description}</div>

          {/* Progress section */}
          {quest.completed ? (
            <div className="mt-2 flex items-center gap-1.5">
              <span className="text-neon-green text-xs font-semibold text-glow-green">
                ✓ Complete
              </span>
            </div>
          ) : (
            <div className="mt-2">
              <div className="flex justify-between items-center mb-1">
                <span className="font-mono text-[10px] text-text-dim">
                  {quest.progress}/{quest.target}
                </span>
                <span className="font-mono text-[10px] text-text-dim">{pct}%</span>
              </div>
              <div className="h-1.5 bg-border-dim rounded-full overflow-hidden">
                <div
                  className="h-full rounded-full bg-neon-blue shadow-neon-blue-sm transition-all duration-500"
                  style={{ width: `${pct}%` }}
                />
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ── Skeleton loader ───────────────────────────────────────────────────────────

function QuestCardSkeleton() {
  return (
    <div className="rounded-lg border border-border-dim bg-bg-card p-3 animate-pulse">
      <div className="flex items-start gap-2.5">
        <div className="w-8 h-8 rounded bg-border-dim flex-shrink-0" />
        <div className="flex-1 space-y-2">
          <div className="flex justify-between">
            <div className="h-3.5 w-1/3 rounded bg-border-dim" />
            <div className="h-3 w-1/4 rounded bg-border-dim" />
          </div>
          <div className="h-2.5 w-2/3 rounded bg-border-dim" />
          <div className="h-1.5 w-full rounded bg-border-dim mt-2" />
        </div>
      </div>
    </div>
  )
}
