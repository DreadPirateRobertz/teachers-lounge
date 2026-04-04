import Link from 'next/link'

export default function AppHeader() {
  return (
    <header className="flex items-center justify-between px-4 h-12 bg-bg-panel border-b border-border-dim flex-shrink-0">
      {/* Logo */}
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-2">
          <span className="font-mono text-sm font-bold text-neon-blue text-glow-blue tracking-widest uppercase">
            TV
          </span>
          <span className="text-xs text-text-dim font-mono tracking-wide hidden sm:block">
            TeachersLounge
          </span>
        </div>
        <Link
          href="/analytics"
          className="hidden sm:flex items-center gap-1 text-[10px] font-mono text-text-dim hover:text-neon-blue transition-colors border border-border-dim rounded-full px-2 py-0.5 hover:border-neon-blue/40"
        >
          <span>📊</span>
          <span>Analytics</span>
        </Link>
      </div>

      {/* Prof Nova status */}
      <div className="flex items-center gap-2 bg-bg-card border border-border-dim rounded-full px-3 py-1">
        <span className="text-sm">🤖</span>
        <span className="text-xs font-medium text-text-base">Prof Nova</span>
        <span className="w-1.5 h-1.5 rounded-full bg-neon-green shadow-neon-green-sm animate-pulse-slow" />
      </div>

      {/* Stats */}
      <div className="flex items-center gap-3">
        <StreakBadge value="7" />
        <StatBadge icon="⚡" value="2.3k" label="xp" color="text-neon-blue" glow />
        <GemBadge value="450" />

        {/* Avatar */}
        <button className="flex items-center gap-1.5 bg-bg-card border border-border-mid rounded-full px-2.5 py-1 hover:border-neon-blue/50 transition-colors">
          <span className="text-base leading-none">🧙</span>
          <span className="text-xs font-mono text-text-dim hidden md:block">Lv 12</span>
        </button>
      </div>
    </header>
  )
}

function StreakBadge({ value }: { value: string }) {
  return (
    <div className="hidden sm:flex items-center gap-1">
      <span className="text-sm leading-none animate-streak-flame">🔥</span>
      <span className="font-mono text-xs font-bold text-orange-400">{value}</span>
    </div>
  )
}

function GemBadge({ value }: { value: string }) {
  return (
    <div className="hidden sm:flex items-center gap-1">
      <span className="text-sm leading-none animate-gem-sparkle">💎</span>
      <span className="font-mono text-xs font-bold text-neon-pink">{value}</span>
    </div>
  )
}

function StatBadge({
  icon,
  value,
  color,
  glow = false,
}: {
  icon: string
  value: string
  label?: string
  color: string
  glow?: boolean
}) {
  return (
    <div className="hidden sm:flex items-center gap-1">
      <span className="text-sm leading-none">{icon}</span>
      <span className={`font-mono text-xs font-bold ${color} ${glow ? 'text-glow-blue' : ''}`}>
        {value}
      </span>
    </div>
  )
}
