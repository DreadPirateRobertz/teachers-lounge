import Link from 'next/link'
import BossProgressionMap from '@/components/boss/BossProgressionMap'

/**
 * ProgressionPage — server component that renders the boss progression map.
 *
 * The BossProgressionMap client component fetches progression data itself
 * on mount; this server component provides the page shell and nav link.
 */
export default function ProgressionPage() {
  return (
    <div className="min-h-screen bg-bg-deep px-4 py-8 flex flex-col items-center">
      <div className="w-full max-w-sm">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-lg font-bold text-text-bright font-mono tracking-wider">
            ⚔ Boss Trail
          </h1>
          <Link href="/" className="text-xs text-neon-blue hover:text-glow-blue transition-colors">
            ← Back
          </Link>
        </div>

        <BossProgressionMap />
      </div>
    </div>
  )
}
