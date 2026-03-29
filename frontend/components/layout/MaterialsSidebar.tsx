// Mock data — will be replaced by Gaming Service / Search Service API calls
const MASTERY_TOPICS = [
  { name: 'Atomic Structure', score: 0.88 },
  { name: 'Chemical Bonding', score: 0.71 },
  { name: 'Nomenclature', score: 0.65 },
  { name: 'Stereochemistry', score: 0.42 },
  { name: 'Organic Reactions', score: 0.29 },
]

const LEADERBOARD = [
  { rank: 1, name: 'MoleMaster', xp: 4820, isRival: true },
  { rank: 2, name: 'ChemWizard', xp: 2340, isMe: true },
  { rank: 3, name: 'BondBreaker', xp: 2100, isRival: true },
  { rank: 4, name: 'NovaStar', xp: 1950, isRival: true },
  { rank: 5, name: 'ReactKing', xp: 1780, isRival: true },
]

const POWERUPS = [
  { icon: '🧠', name: 'Hint', desc: 'Remove wrong answer', cost: 50, owned: 3 },
  { icon: '🛡️', name: 'Shield', desc: 'Block one hit', cost: 75, owned: 1 },
  { icon: '⚡', name: '2× Damage', desc: 'Double XP next answer', cost: 100, owned: 0 },
  { icon: '⏰', name: 'Time+', desc: '+15 sec to timer', cost: 60, owned: 2 },
]

function masteryColor(score: number) {
  if (score >= 0.9) return 'bg-neon-gold'
  if (score >= 0.7) return 'bg-neon-green'
  if (score >= 0.4) return 'bg-yellow-400'
  return 'bg-red-500'
}

function masteryLabel(score: number) {
  if (score >= 0.9) return 'text-neon-gold'
  if (score >= 0.7) return 'text-neon-green'
  if (score >= 0.4) return 'text-yellow-400'
  return 'text-red-400'
}

export default function MaterialsSidebar() {
  // Using plain HTML state simulation — client component needed for real tabs
  // For Phase 1 we render mastery by default (static server component is fine)
  return (
    <aside className="w-[280px] flex-shrink-0 flex flex-col bg-bg-panel overflow-hidden">
      {/* Tab bar */}
      <div className="flex border-b border-border-dim flex-shrink-0">
        <SidebarTab label="Mastery" active />
        <SidebarTab label="Rankings" />
        <SidebarTab label="Power-ups" />
      </div>

      {/* Mastery panel (default) */}
      <div className="flex-1 overflow-y-auto p-3">
        <div className="text-[10px] font-bold text-text-dim uppercase tracking-wider mb-3">
          Topic Mastery
        </div>
        <div className="flex flex-col gap-2">
          {MASTERY_TOPICS.map((t) => (
            <div key={t.name}>
              <div className="flex justify-between text-xs mb-1">
                <span className="text-text-base truncate pr-2">{t.name}</span>
                <span className={`font-mono font-bold flex-shrink-0 ${masteryLabel(t.score)}`}>
                  {Math.round(t.score * 100)}%
                </span>
              </div>
              <div className="h-1.5 bg-border-dim rounded-full overflow-hidden">
                <div
                  className={`h-full rounded-full transition-all ${masteryColor(t.score)}`}
                  style={{ width: `${t.score * 100}%` }}
                />
              </div>
            </div>
          ))}
        </div>

        {/* Leaderboard preview */}
        <div className="mt-5">
          <div className="text-[10px] font-bold text-text-dim uppercase tracking-wider mb-3">
            Rankings
          </div>
          <div className="flex flex-col gap-1">
            {LEADERBOARD.map((entry) => (
              <div
                key={entry.rank}
                className={`flex items-center gap-2 px-2 py-1.5 rounded text-xs ${
                  entry.isMe
                    ? 'bg-neon-blue/10 border border-neon-blue/30'
                    : 'bg-bg-card border border-border-dim'
                }`}
              >
                <span className={`font-mono font-bold w-4 flex-shrink-0 ${entry.rank === 1 ? 'text-neon-gold' : 'text-text-dim'}`}>
                  {entry.rank}
                </span>
                <span className={`flex-1 truncate ${entry.isMe ? 'text-neon-blue font-medium' : 'text-text-base'}`}>
                  {entry.name}
                  {entry.isMe && ' (you)'}
                  {entry.isRival && !entry.isMe && (
                    <span className="text-text-dim ml-1 text-[10px]">rival</span>
                  )}
                </span>
                <span className="font-mono text-[10px] text-text-dim flex-shrink-0">
                  {entry.xp.toLocaleString()}
                </span>
              </div>
            ))}
          </div>
        </div>

        {/* Power-ups preview */}
        <div className="mt-5">
          <div className="text-[10px] font-bold text-text-dim uppercase tracking-wider mb-3">
            Power-ups
          </div>
          <div className="flex flex-col gap-1.5">
            {POWERUPS.map((p) => (
              <div key={p.name} className="flex items-center gap-2 bg-bg-card border border-border-dim rounded p-2">
                <span className="text-lg leading-none">{p.icon}</span>
                <div className="flex-1 min-w-0">
                  <div className="text-xs text-text-base font-medium">{p.name}</div>
                  <div className="text-[10px] text-text-dim">{p.desc}</div>
                </div>
                <div className="flex flex-col items-end flex-shrink-0">
                  <span className="font-mono text-[10px] text-neon-pink">💎{p.cost}</span>
                  <span className="font-mono text-[10px] text-text-dim">×{p.owned}</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </aside>
  )
}

function SidebarTab({ label, active = false }: { label: string; active?: boolean }) {
  return (
    <div
      className={`flex-1 text-center py-2 text-[11px] font-medium border-b-2 transition-colors ${
        active
          ? 'text-neon-blue border-neon-blue'
          : 'text-text-dim border-transparent hover:text-text-base'
      }`}
    >
      {label}
    </div>
  )
}
