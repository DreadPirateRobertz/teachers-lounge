'use client'

interface Props {
  current: number
  levelMax: number
  level: number
  onLevelUp?: () => void
}

export default function XpProgressBar({ current, levelMax, level }: Props) {
  const pct = Math.min(100, Math.round((current / levelMax) * 100))

  return (
    <div className="flex items-center gap-3 px-4 py-2 bg-bg-panel border-t border-border-dim flex-shrink-0">
      {/* Level badge */}
      <div className="flex-shrink-0 flex items-center justify-center w-8 h-8 rounded-full bg-bg-card border border-neon-blue/40 shadow-neon-blue-sm">
        <span className="font-mono text-[11px] font-bold text-neon-blue text-glow-blue leading-none">
          {level}
        </span>
      </div>

      {/* Progress bar */}
      <div className="flex-1 relative h-4 bg-border-dim rounded-full overflow-hidden border border-border-mid">
        {/* Fill */}
        <div
          className="absolute inset-y-0 left-0 rounded-full bg-neon-blue transition-all duration-700 ease-out"
          style={{
            width: `${pct}%`,
            boxShadow: '0 0 6px #00aaff99, 0 0 12px #00aaff44',
          }}
        >
          {/* Shimmer sweep */}
          <div
            className="absolute inset-y-0 w-1/3 rounded-full"
            style={{
              background:
                'linear-gradient(90deg, transparent 0%, rgba(255,255,255,0.35) 50%, transparent 100%)',
              animation: 'xp-shimmer 2.8s ease-in-out infinite',
            }}
          />
        </div>

        {/* Glowing edge dot at fill tip */}
        {pct > 2 && pct < 99 && (
          <div
            className="absolute top-1/2 -translate-y-1/2 w-2 h-2 rounded-full bg-white animate-xp-edge"
            style={{ left: `calc(${pct}% - 4px)` }}
          />
        )}

        {/* Percentage label */}
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
          <span className="font-mono text-[10px] font-bold text-white/80 mix-blend-overlay select-none">
            {pct}%
          </span>
        </div>
      </div>

      {/* Next level label */}
      <div className="flex-shrink-0 text-right">
        <div className="font-mono text-[10px] text-neon-blue">
          {current.toLocaleString()} / {levelMax.toLocaleString()}
        </div>
        <div className="font-mono text-[9px] text-text-dim">→ Lv {level + 1}</div>
      </div>
    </div>
  )
}
