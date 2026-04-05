import AppHeader from '@/components/layout/AppHeader'
import XpProgressBar from '@/components/layout/XpProgressBar'
import QuestBoard from '@/components/quests/QuestBoard'
import Link from 'next/link'

export default function QuestsPage() {
  return (
    <div className="flex flex-col h-screen bg-bg-deep overflow-hidden">
      <AppHeader />

      <div className="flex-1 overflow-y-auto px-4 py-6 max-w-2xl mx-auto w-full">
        {/* Back nav */}
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 text-xs text-text-dim hover:text-neon-blue transition-colors mb-6"
        >
          <span>←</span>
          <span>Back to dashboard</span>
        </Link>

        {/* Streak multiplier banner */}
        <StreakBanner streak={7} multiplier={2.0} />

        {/* Quest board */}
        <div className="mt-4">
          <QuestBoard />
        </div>

        {/* Motivational note */}
        <p className="mt-6 text-[11px] text-text-dim text-center italic">
          Quests reset every day at midnight UTC. Come back tomorrow for new challenges!
        </p>
      </div>

      <XpProgressBar current={2340} levelMax={3000} level={12} />
    </div>
  )
}

// ── Streak multiplier banner ──────────────────────────────────────────────────

function StreakBanner({ streak, multiplier }: { streak: number; multiplier: number }) {
  const pct = Math.min(100, (streak / 30) * 100)

  return (
    <div className="bg-bg-card border border-orange-500/25 rounded-lg px-4 py-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span
            className="text-xl leading-none animate-streak-flame"
            style={{ display: 'inline-block' }}
          >
            🔥
          </span>
          <div>
            <div className="text-sm font-semibold text-orange-400">{streak}-Day Streak Active</div>
            <div className="text-[11px] text-text-dim">
              Keep it going — don&apos;t break the chain!
            </div>
          </div>
        </div>
        <div className="text-right">
          <div className="font-mono text-lg font-bold text-neon-gold text-glow-gold">
            ×{multiplier.toFixed(1)}
          </div>
          <div className="text-[10px] text-text-dim uppercase tracking-wider">XP multiplier</div>
        </div>
      </div>
      {/* Heat bar */}
      <div className="mt-2.5 h-1 bg-border-dim rounded-full overflow-hidden">
        <div
          className="h-full rounded-full transition-all"
          style={{
            width: `${pct}%`,
            background: 'linear-gradient(90deg, #ff6600, #ff4400)',
            boxShadow: '0 0 6px #ff440099',
          }}
        />
      </div>
      <div className="flex justify-between mt-1">
        <span className="text-[10px] text-text-dim font-mono">{streak} days</span>
        <span className="text-[10px] text-text-dim font-mono">30 days</span>
      </div>
    </div>
  )
}
