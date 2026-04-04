interface StatCardProps {
  label: string
  value: string | number
  sub?: string
  color?: 'blue' | 'green' | 'pink' | 'gold'
}

const colorMap = {
  blue: 'text-neon-blue text-glow-blue border-neon-blue/30',
  green: 'text-neon-green text-glow-green border-neon-green/30',
  pink: 'text-neon-pink border-neon-pink/30',
  gold: 'text-neon-gold border-neon-gold/30',
} as const

export default function StatCard({ label, value, sub, color = 'blue' }: StatCardProps) {
  const cls = colorMap[color]
  return (
    <div className={`bg-bg-card border rounded-xl p-4 flex flex-col gap-1 ${cls.split(' ')[2]}`}>
      <span className="text-[10px] uppercase tracking-widest text-text-dim font-mono">{label}</span>
      <span className={`font-mono text-2xl font-bold ${cls.split(' ').slice(0, 2).join(' ')}`}>
        {value}
      </span>
      {sub && <span className="text-xs text-text-dim">{sub}</span>}
    </div>
  )
}
