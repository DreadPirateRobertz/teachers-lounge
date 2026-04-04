'use client'

interface TopicStat {
  topic: string
  total: number
  correct: number
  accuracy_pct: number
}

interface QuizBreakdownChartProps {
  topics: TopicStat[]
}

export default function QuizBreakdownChart({ topics }: QuizBreakdownChartProps) {
  if (topics.length === 0) {
    return (
      <div className="text-xs text-text-dim text-center py-8">
        No quiz data yet. Complete some quizzes to see your breakdown.
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-3">
      {topics.map(t => (
        <div key={t.topic} className="flex flex-col gap-1">
          <div className="flex items-center justify-between">
            <span className="text-xs text-text-base truncate max-w-[55%]">{t.topic}</span>
            <span className="text-xs font-mono text-text-dim">
              {t.correct}/{t.total} &middot;{' '}
              <span
                className={
                  t.accuracy_pct >= 80
                    ? 'text-neon-green'
                    : t.accuracy_pct >= 60
                      ? 'text-neon-gold'
                      : 'text-neon-pink'
                }
              >
                {t.accuracy_pct}%
              </span>
            </span>
          </div>
          {/* Track */}
          <div className="h-2 bg-bg-panel rounded-full overflow-hidden">
            {/* Correct portion */}
            <div
              className="h-full rounded-full transition-all duration-500"
              style={{
                width: `${t.accuracy_pct}%`,
                background:
                  t.accuracy_pct >= 80
                    ? '#00ff88'
                    : t.accuracy_pct >= 60
                      ? '#ffdc00'
                      : '#ff00aa',
                boxShadow:
                  t.accuracy_pct >= 80
                    ? '0 0 6px #00ff8866'
                    : t.accuracy_pct >= 60
                      ? '0 0 6px #ffdc0066'
                      : '0 0 6px #ff00aa66',
              }}
            />
          </div>
        </div>
      ))}
    </div>
  )
}
