import Link from 'next/link'

export default function SubscribeCancelPage() {
  return (
    <div className="min-h-screen bg-bg-deep flex flex-col items-center justify-center px-4 text-center">
      <div className="text-6xl mb-6 opacity-70">🌀</div>

      <h1 className="font-mono text-xl font-bold text-text-bright mb-2">
        No worries.
      </h1>
      <p className="text-sm text-text-dim mb-8 max-w-xs">
        You can subscribe whenever you&apos;re ready. Your account is waiting.
      </p>

      <div className="flex flex-col gap-3 w-full max-w-xs">
        <Link
          href="/subscribe"
          className="px-6 py-3 bg-neon-blue text-bg-deep font-semibold rounded-lg shadow-neon-blue-sm hover:bg-neon-blue/90 transition-colors text-sm text-center"
        >
          Try again ⚡
        </Link>
        <Link
          href="/"
          className="px-6 py-3 border border-border-mid text-text-dim rounded-lg hover:border-neon-blue/30 hover:text-text-base transition-colors text-sm text-center"
        >
          Go to dashboard
        </Link>
      </div>

      <p className="mt-8 text-[10px] text-text-dim italic">
        &ldquo;It does not matter how slowly you go as long as you do not stop.&rdquo; — Confucius
      </p>
    </div>
  )
}
