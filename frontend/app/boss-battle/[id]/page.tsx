import Link from 'next/link'
import BossBattleClient from '@/components/boss/BossBattleClient'
import { getBossDef } from '@/components/boss/BossCharacterLibrary'

/**
 * BossBattlePage — server component that resolves boss metadata and renders
 * the interactive BossBattleClient.
 *
 * The `id` param maps to a boss ID from BossCharacterLibrary
 * (e.g. "the_atom", "the_bonder", "final_boss").
 * Chapter IDs (numbers 1-6) are also accepted for legacy URL support.
 */
export default async function BossBattlePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params

  // Accept either the string ID ("the_atom") or the legacy chapter number ("1").
  const bossId = resolveBossId(id)
  const boss = getBossDef(bossId)

  if (!boss) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen bg-bg-deep text-center px-4">
        <h1 className="font-mono text-xl font-bold text-neon-pink mb-4">Boss not found</h1>
        <p className="text-sm text-text-dim font-mono mb-6">
          No boss found for id: <code className="text-neon-blue">{id}</code>
        </p>
        <Link
          href="/"
          className="text-xs text-neon-blue border border-neon-blue/30 px-4 py-2 rounded-lg hover:bg-neon-blue/10 transition-colors"
        >
          ← Back to Tutor
        </Link>
      </div>
    )
  }

  // userId and initialGems would normally come from the session cookie / auth token.
  // For now we pass placeholder values — the real auth integration is Phase 2 work.
  const userId = 'demo-user'
  const initialGems = 10

  return <BossBattleClient boss={boss} userId={userId} initialGems={initialGems} />
}

/**
 * Maps a URL segment to a canonical boss ID.
 * Accepts numeric chapter IDs (1–6) as well as string IDs.
 */
function resolveBossId(segment: string): string {
  const chapterMap: Record<string, string> = {
    '1': 'the_atom',
    '2': 'the_bonder',
    '3': 'name_lord',
    '4': 'the_stereochemist',
    '5': 'the_reactor',
    '6': 'final_boss',
  }
  return chapterMap[segment] ?? segment
}
