import Link from 'next/link'

export default function SubscribeSuccessPage() {
  return (
    <div className="min-h-screen bg-bg-deep flex flex-col items-center justify-center px-4 text-center">
      {/* CSS confetti burst */}
      <div className="relative mb-8">
        <div className="text-7xl animate-bounce-slow">🎓</div>
        <Confetti />
      </div>

      <h1 className="font-mono text-2xl font-bold text-neon-green text-glow-green mb-2">
        Welcome to TeachersLounge!
      </h1>
      <p className="text-sm text-text-base mb-1">Your 14-day free trial is now active.</p>
      <p className="text-xs text-text-dim mb-8">No charge until your trial ends. Cancel anytime.</p>

      <Link
        href="/"
        className="px-6 py-3 bg-neon-green text-bg-deep font-semibold rounded-lg shadow-neon-green-sm hover:bg-neon-green/90 transition-colors text-sm"
      >
        Start Learning ⚡
      </Link>

      <p className="mt-6 text-[10px] text-text-dim italic">
        &ldquo;The more that you read, the more things you will know.&rdquo; — Dr. Seuss
      </p>
    </div>
  )
}

function Confetti() {
  const pieces = [
    {
      color: 'bg-neon-blue',
      x: '-translate-x-16',
      y: '-translate-y-12',
      delay: '0ms',
      size: 'w-2 h-2',
    },
    {
      color: 'bg-neon-pink',
      x: 'translate-x-14',
      y: '-translate-y-16',
      delay: '100ms',
      size: 'w-1.5 h-3',
    },
    {
      color: 'bg-neon-gold',
      x: '-translate-x-8',
      y: '-translate-y-20',
      delay: '200ms',
      size: 'w-2.5 h-1.5',
    },
    {
      color: 'bg-neon-green',
      x: 'translate-x-18',
      y: '-translate-y-10',
      delay: '50ms',
      size: 'w-1.5 h-2',
    },
    {
      color: 'bg-neon-blue',
      x: 'translate-x-6',
      y: '-translate-y-24',
      delay: '150ms',
      size: 'w-2 h-1.5',
    },
    {
      color: 'bg-neon-pink',
      x: '-translate-x-20',
      y: '-translate-y-8',
      delay: '250ms',
      size: 'w-1.5 h-2.5',
    },
    {
      color: 'bg-neon-gold',
      x: 'translate-x-10',
      y: '-translate-y-20',
      delay: '75ms',
      size: 'w-2 h-2',
    },
    {
      color: 'bg-neon-green',
      x: '-translate-x-12',
      y: '-translate-y-16',
      delay: '175ms',
      size: 'w-1.5 h-1.5',
    },
  ]

  return (
    <div className="absolute inset-0 pointer-events-none">
      {pieces.map((p, i) => (
        <div
          key={i}
          className={`absolute left-1/2 top-1/2 ${p.size} ${p.color} rounded-sm opacity-80 animate-confetti ${p.x} ${p.y}`}
          style={{ animationDelay: p.delay }}
        />
      ))}
    </div>
  )
}
