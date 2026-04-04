import Link from 'next/link'

// Phase 4 stub — WebGL/Three.js boss battles are built in Phase 4
// This page is the future home of the animated boss fight experience.
//
// What will live here:
//   - Three.js canvas (molecule bosses, physics via Cannon.js/Rapier)
//   - HP bars (student + boss), timer, power-up tray
//   - 5-7 quiz rounds with escalating difficulty
//   - Combo system, loot drops, Weird Science particle effects
//   - Sci-fi quote overlays on hits/misses

export default async function BossBattlePage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const { id } = await params
  return (
    <div className="flex flex-col items-center justify-center min-h-screen bg-bg-deep text-center px-4">
      {/* Boss placeholder art */}
      <div className="relative mb-8">
        <div className="text-8xl animate-pulse-slow">⚗️</div>
        <div className="absolute inset-0 rounded-full bg-neon-pink/5 blur-2xl" />
      </div>

      <h1 className="font-mono text-2xl font-bold text-neon-pink text-glow-pink mb-2">
        Boss Battle
      </h1>
      <p className="text-xs font-mono text-text-dim mb-1">Chapter {id}</p>

      <div className="mt-6 px-6 py-4 bg-bg-card border border-neon-pink/20 rounded-xl max-w-sm">
        <p className="text-sm text-text-base mb-2">
          ⚔️ <strong className="text-text-bright">The Atom</strong> awaits.
        </p>
        <p className="text-xs text-text-dim leading-relaxed">
          Boss battles arrive in <span className="text-neon-gold font-mono">Phase 4</span>.
          Finish studying your course material — the fight will unlock when you
          reach 60% mastery on this chapter.
        </p>
      </div>

      <div className="mt-4 text-xs text-text-dim italic">
        &ldquo;Do. Or do not. There is no try.&rdquo; — Yoda
      </div>

      <Link
        href="/"
        className="mt-8 text-xs text-neon-blue border border-neon-blue/30 px-4 py-2 rounded-lg hover:bg-neon-blue/10 transition-colors"
      >
        ← Back to Tutor
      </Link>
    </div>
  )
}
