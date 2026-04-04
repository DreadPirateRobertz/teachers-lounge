'use client'

interface DayActivity {
  date: string
  messages: number
}

interface ActivityChartProps {
  days: DayActivity[]
}

export default function ActivityChart({ days }: ActivityChartProps) {
  const max = Math.max(...days.map(d => d.messages), 1)

  // Show last 30 days as a grid of squares (GitHub-style heatmap)
  return (
    <div className="flex flex-col gap-2">
      <div className="flex gap-1 flex-wrap">
        {days.map(d => {
          const intensity = d.messages / max
          const opacity = d.messages === 0 ? 0.08 : 0.2 + intensity * 0.8
          return (
            <div
              key={d.date}
              title={`${d.date}: ${d.messages} message${d.messages !== 1 ? 's' : ''}`}
              className="w-5 h-5 rounded-sm cursor-default transition-all"
              style={{
                background: `rgba(0, 170, 255, ${opacity})`,
                boxShadow: d.messages > 0 ? `0 0 4px rgba(0, 170, 255, ${opacity * 0.6})` : 'none',
              }}
            />
          )
        })}
      </div>
      <div className="flex items-center gap-2 text-[10px] text-text-dim font-mono">
        <span>Less</span>
        {[0.08, 0.3, 0.55, 0.8, 1].map(o => (
          <div
            key={o}
            className="w-3 h-3 rounded-sm"
            style={{ background: `rgba(0, 170, 255, ${o})` }}
          />
        ))}
        <span>More</span>
      </div>
    </div>
  )
}
