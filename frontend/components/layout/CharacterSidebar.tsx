'use client'

import Link from 'next/link'
import { useEffect, useState } from 'react'

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

const MOCK_ACHIEVEMENTS = ['🧪', '🔥', '⭐', '🏆', '💡', '🎯']

export default function CharacterSidebar() {
  const [quests, setQuests] = useState<QuestState[]>([])

  useEffect(() => {
    fetch('/api/gaming/quests', { cache: 'no-store' })
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (data?.quests) setQuests(data.quests)
      })
      .catch(() => undefined)
  }, [])

  return (
    <aside className="w-[220px] flex-shrink-0 flex flex-col bg-bg-panel overflow-y-auto">
      {/* Avatar card */}
      <div className="p-3 border-b border-border-dim">
        <div className="bg-bg-card border border-border-mid rounded-lg p-3 text-center">
          <div className="text-4xl mb-1">🧙</div>
          <div className="font-mono text-xs text-neon-blue text-glow-blue font-bold">
            ChemWizard
          </div>
          <div className="text-[10px] text-text-dim mt-0.5">Scholar · Rank IV</div>

          {/* XP mini bar */}
          <div className="mt-2">
            <div className="flex justify-between text-[10px] text-text-dim mb-1">
              <span>Lv 12</span>
              <span className="font-mono text-neon-blue">2340 / 3000</span>
            </div>
            <div className="h-1 bg-border-dim rounded-full overflow-hidden">
              <div
                className="h-full rounded-full bg-neon-blue shadow-neon-blue-sm transition-all"
                style={{ width: '78%' }}
              />
            </div>
          </div>
        </div>
      </div>

      {/* Streak */}
      <div className="px-3 py-2 border-b border-border-dim">
        <div className="flex items-center justify-between">
          <span className="text-xs text-text-dim">Daily Streak</span>
          <div className="flex items-center gap-0.5">
            <span
              className="text-base leading-none animate-streak-flame"
              style={{ animationDelay: '0s' }}
            >
              🔥
            </span>
            <span
              className="text-sm leading-none animate-streak-flame"
              style={{ animationDelay: '0.2s', opacity: 0.7 }}
            >
              🔥
            </span>
            <span
              className="text-xs leading-none animate-streak-flame"
              style={{ animationDelay: '0.4s', opacity: 0.4 }}
            >
              🔥
            </span>
            <span className="font-mono text-sm font-bold text-orange-400 ml-1">7</span>
          </div>
        </div>
        <div className="flex items-center justify-between mt-1">
          <div className="text-[10px] text-text-dim">2× XP multiplier active</div>
          <div className="text-[10px] font-mono text-orange-400/70">days</div>
        </div>
        {/* Streak heat bar */}
        <div className="mt-1.5 h-1 bg-border-dim rounded-full overflow-hidden">
          <div
            className="h-full rounded-full transition-all"
            style={{
              width: '70%',
              background: 'linear-gradient(90deg, #ff6600, #ff4400)',
              boxShadow: '0 0 4px #ff440099',
            }}
          />
        </div>
      </div>

      {/* Daily Quests */}
      <div className="px-3 py-2 border-b border-border-dim">
        <div className="flex items-center justify-between mb-2">
          <div className="text-[10px] font-bold text-text-dim uppercase tracking-wider">
            Daily Quests
          </div>
          <Link
            href="/quests"
            className="text-[10px] text-neon-blue hover:text-glow-blue transition-colors"
          >
            View all →
          </Link>
        </div>
        <div className="flex flex-col gap-1.5">
          {quests.length > 0
            ? quests.map((q) => <QuestItem key={q.id} quest={q} />)
            : SIDEBAR_MOCK_QUESTS.map((q) => <QuestItem key={q.id} quest={q} />)}
        </div>
      </div>

      {/* Achievements */}
      <div className="px-3 py-2 border-b border-border-dim">
        <div className="text-[10px] font-bold text-text-dim uppercase tracking-wider mb-2">
          Achievements
        </div>
        <div className="flex flex-wrap gap-1.5">
          {MOCK_ACHIEVEMENTS.map((icon, i) => (
            <div
              key={i}
              className="w-7 h-7 flex items-center justify-center bg-bg-card border border-border-dim rounded text-sm hover:border-neon-gold/50 transition-colors cursor-default"
              title="Achievement"
            >
              {icon}
            </div>
          ))}
          <div className="w-7 h-7 flex items-center justify-center bg-bg-card border border-dashed border-border-dim rounded text-text-dim text-xs">
            ?
          </div>
        </div>
      </div>

      {/* Materials list (stub) */}
      <div className="px-3 py-2">
        <div className="text-[10px] font-bold text-text-dim uppercase tracking-wider mb-2">
          My Courses
        </div>
        <div className="text-xs text-text-dim italic">No materials yet.</div>
        <button className="mt-2 w-full text-xs font-medium text-neon-green border border-neon-green/30 rounded px-2 py-1.5 hover:bg-neon-green/10 transition-colors">
          + Upload Material
        </button>
      </div>
    </aside>
  )
}

// ── Sidebar mock quests (shown while API loads) ───────────────────────────────

const SIDEBAR_MOCK_QUESTS = [
  {
    id: 'questions_answered',
    title: 'Question Seeker',
    description: 'Answer 5 questions today',
    progress: 3,
    target: 5,
    completed: false,
    xp_reward: 25,
    gems_reward: 5,
  },
  {
    id: 'keep_streak_alive',
    title: 'Streak Keeper',
    description: 'Keep your learning streak alive',
    progress: 1,
    target: 1,
    completed: true,
    xp_reward: 35,
    gems_reward: 10,
  },
  {
    id: 'master_new_concept',
    title: 'Concept Pioneer',
    description: 'Master a new concept',
    progress: 0,
    target: 1,
    completed: false,
    xp_reward: 75,
    gems_reward: 20,
  },
]

// ── Quest item (compact sidebar display) ─────────────────────────────────────

function QuestItem({ quest }: { quest: (typeof SIDEBAR_MOCK_QUESTS)[0] }) {
  const pct = quest.target > 0 ? Math.round((quest.progress / quest.target) * 100) : 0

  return (
    <div
      className={`rounded p-1.5 ${
        quest.completed
          ? 'bg-neon-green/10 border border-neon-green/20'
          : 'bg-bg-card border border-border-dim'
      }`}
    >
      <div className="flex items-start justify-between gap-1">
        <span className="text-[11px] text-text-base leading-tight">{quest.title}</span>
        <span className="font-mono text-[10px] text-neon-gold flex-shrink-0">
          +{quest.xp_reward}xp
        </span>
      </div>
      {!quest.completed && (
        <div className="mt-1">
          <div className="h-0.5 bg-border-dim rounded-full overflow-hidden">
            <div
              className="h-full rounded-full bg-neon-blue transition-all"
              style={{ width: `${pct}%` }}
            />
          </div>
          <div className="text-[10px] text-text-dim mt-0.5 font-mono">
            {quest.progress}/{quest.target}
          </div>
        </div>
      )}
      {quest.completed && <div className="text-[10px] text-neon-green mt-0.5">✓ Complete</div>}
    </div>
  )
}
