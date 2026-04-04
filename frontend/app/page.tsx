import AppHeader from '@/components/layout/AppHeader'
import CharacterSidebar from '@/components/layout/CharacterSidebar'
import MaterialsSidebar from '@/components/layout/MaterialsSidebar'
import XpProgressBar from '@/components/layout/XpProgressBar'
import LevelUpBanner from '@/components/layout/LevelUpBanner'
import ChatPanel from '@/components/chat/ChatPanel'

export default function HomePage() {
  return (
    <div className="flex flex-col h-screen bg-bg-deep overflow-hidden">
      {/* Top nav */}
      <AppHeader />

      {/* Three-panel body */}
      <div className="flex flex-1 overflow-hidden min-h-0">
        {/* Left: Character & Quests */}
        <CharacterSidebar />

        {/* Center: Chat */}
        <main className="flex-1 flex flex-col min-w-0 border-x border-border-dim">
          <ChatPanel />
        </main>

        {/* Right: Materials / Mastery / Leaderboard */}
        <MaterialsSidebar />
      </div>

      {/* Bottom: XP progress bar */}
      <XpProgressBar current={2340} levelMax={3000} level={12} />

      {/* Level-up overlay — shown on mount for demo; wire to gaming events in production */}
      <LevelUpBanner newLevel={13} />
    </div>
  )
}
