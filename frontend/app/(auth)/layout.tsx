export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-bg-deep flex flex-col items-center justify-center px-4">
      {/* Logo */}
      <div className="mb-8 text-center">
        <div className="font-mono text-2xl font-bold text-neon-blue text-glow-blue tracking-widest uppercase mb-1">
          TeachersLounge
        </div>
        <div className="text-xs text-text-dim font-mono tracking-wide">
          AI-Powered Personalized Tutor
        </div>
      </div>

      {children}

      {/* Footer quote */}
      <div className="mt-8 text-center">
        <p className="text-xs text-text-dim italic max-w-xs">
          &ldquo;The more I learn, the more I realize how much I don&apos;t know.&rdquo;
        </p>
        <p className="text-[10px] text-border-mid mt-1">— Albert Einstein</p>
      </div>
    </div>
  )
}
