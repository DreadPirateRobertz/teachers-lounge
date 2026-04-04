// Mock data — will be replaced by User Service / Gaming Service API calls
const MOCK_QUESTS = [
  { id: 1, label: 'Ask 5 questions', progress: 3, total: 5, xp: 25 },
  { id: 2, label: 'Keep streak alive', progress: 1, total: 1, xp: 50, done: true },
  { id: 3, label: 'Master 1 concept', progress: 0, total: 1, xp: 75 },
]

const MOCK_ACHIEVEMENTS = ['🧪', '🔥', '⭐', '🏆', '💡', '🎯']

export default function CharacterSidebar() {
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
            {/* Three flames that stagger slightly */}
            <span className="text-base leading-none animate-streak-flame" style={{ animationDelay: '0s' }}>🔥</span>
            <span className="text-sm leading-none animate-streak-flame" style={{ animationDelay: '0.2s', opacity: 0.7 }}>🔥</span>
            <span className="text-xs leading-none animate-streak-flame" style={{ animationDelay: '0.4s', opacity: 0.4 }}>🔥</span>
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
        <div className="text-[10px] font-bold text-text-dim uppercase tracking-wider mb-2">
          Daily Quests
        </div>
        <div className="flex flex-col gap-1.5">
          {MOCK_QUESTS.map((q) => (
            <QuestItem key={q.id} quest={q} />
          ))}
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

function QuestItem({ quest }: { quest: typeof MOCK_QUESTS[0] }) {
  const pct = Math.round((quest.progress / quest.total) * 100)
  const done = 'done' in quest && quest.done
  return (
    <div className={`rounded p-1.5 ${done ? 'bg-neon-green/10 border border-neon-green/20' : 'bg-bg-card border border-border-dim'}`}>
      <div className="flex items-start justify-between gap-1">
        <span className="text-[11px] text-text-base leading-tight">{quest.label}</span>
        <span className="font-mono text-[10px] text-neon-gold flex-shrink-0">+{quest.xp}xp</span>
      </div>
      {!done && (
        <div className="mt-1">
          <div className="h-0.5 bg-border-dim rounded-full overflow-hidden">
            <div
              className="h-full rounded-full bg-neon-blue transition-all"
              style={{ width: `${pct}%` }}
            />
          </div>
          <div className="text-[10px] text-text-dim mt-0.5 font-mono">
            {quest.progress}/{quest.total}
          </div>
        </div>
      )}
      {done && <div className="text-[10px] text-neon-green mt-0.5">✓ Complete</div>}
    </div>
  )
}
