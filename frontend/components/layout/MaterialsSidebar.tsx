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

        {/* Power-ups inventory */}
        <div className="mt-5">
          <div className="flex items-center justify-between mb-3">
            <div className="text-[10px] font-bold text-text-dim uppercase tracking-wider">
              Power-ups
            </div>
            <div className="flex items-center gap-1 text-[10px] text-neon-pink font-mono">
              <span className="animate-gem-sparkle inline-block">💎</span>
              <span>450</span>
            </div>
          </div>
          <div className="flex flex-col gap-2">
            {POWERUPS.map((p) => (
              <PowerUpItem key={p.name} powerup={p} />
            ))}
          </div>
        </div>
      </div>
    </aside>
  )
}

function PowerUpItem({ powerup }: { powerup: typeof POWERUPS[0] }) {
  const hasOwned = powerup.owned > 0
  return (
    <div
      className={`rounded-lg border p-2 transition-colors ${
        hasOwned
          ? 'bg-bg-card border-border-mid hover:border-neon-green/40'
          : 'bg-bg-card border-border-dim opacity-70'
      }`}
      style={hasOwned ? { boxShadow: 'inset 0 0 12px #00ff8808' } : undefined}
    >
      <div className="flex items-start gap-2">
        {/* Icon with owned indicator */}
        <div className="relative flex-shrink-0">
          <span className="text-xl leading-none">{powerup.icon}</span>
          {hasOwned && (
            <span
              className="absolute -top-1 -right-1 w-3.5 h-3.5 rounded-full bg-neon-green text-bg-deep font-mono font-black text-[8px] flex items-center justify-center animate-owned-pulse"
            >
              {powerup.owned}
            </span>
          )}
        </div>

        {/* Name + desc */}
        <div className="flex-1 min-w-0">
          <div className={`text-xs font-medium ${hasOwned ? 'text-text-bright' : 'text-text-base'}`}>
            {powerup.name}
          </div>
          <div className="text-[10px] text-text-dim leading-tight">{powerup.desc}</div>
        </div>

        {/* Cost / use button */}
        <div className="flex-shrink-0 flex flex-col items-end gap-1">
          {hasOwned ? (
            <button
              className="text-[10px] font-mono font-bold text-neon-green border border-neon-green/30 rounded px-1.5 py-0.5 hover:bg-neon-green/10 transition-colors"
            >
              USE
            </button>
          ) : (
            <button
              className="text-[10px] font-mono font-bold text-neon-pink border border-neon-pink/30 rounded px-1.5 py-0.5 hover:bg-neon-pink/10 transition-colors"
            >
              BUY
            </button>
          )}
          <span className="font-mono text-[9px] text-text-dim">💎{powerup.cost}</span>
        </div>
      </div>
    </div>
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
