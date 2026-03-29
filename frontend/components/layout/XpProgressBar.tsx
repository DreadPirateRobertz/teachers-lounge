interface Props {
  current: number
  levelMax: number
  level: number
}

export default function XpProgressBar({ current, levelMax, level }: Props) {
  const pct = Math.round((current / levelMax) * 100)

  return (
    <div className="flex items-center gap-3 px-4 py-2 bg-bg-panel border-t border-border-dim flex-shrink-0">
      <span className="font-mono text-xs text-text-dim flex-shrink-0">Lv {level}</span>
      <div className="flex-1 relative h-3 bg-border-dim rounded-full overflow-hidden">
        <div
          className="absolute inset-y-0 left-0 rounded-full bg-neon-blue shadow-neon-blue-sm transition-all duration-500"
          style={{ width: `${pct}%` }}
        />
        <div className="absolute inset-0 flex items-center justify-center">
          <span className="font-mono text-[10px] text-bg-deep font-bold mix-blend-overlay">
            {pct}% to Level {level + 1}
          </span>
        </div>
      </div>
      <span className="font-mono text-xs text-neon-blue flex-shrink-0">
        {current.toLocaleString()} / {levelMax.toLocaleString()} XP
      </span>
    </div>
  )
}
