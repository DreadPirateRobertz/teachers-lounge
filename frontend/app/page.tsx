import AppHeader from '@/components/layout/AppHeader'
import BottomNav from '@/components/layout/BottomNav'
import CharacterSidebar from '@/components/layout/CharacterSidebar'
import MaterialsSidebar from '@/components/layout/MaterialsSidebar'
import XpProgressBar from '@/components/layout/XpProgressBar'
import LevelUpBanner from '@/components/layout/LevelUpBanner'
import ChatPanel from '@/components/chat/ChatPanel'

export default function HomePage() {
  return (
    /*
     * Layout strategy:
     *   Mobile  (<md): single-column — sidebars hidden, BottomNav shown.
     *   Desktop (≥md): three-panel — both sidebars visible, BottomNav hidden.
     *
     * `pb-14` on mobile creates clearance for the 56px fixed BottomNav so
     * content is not obscured at the bottom of the viewport.
     */
    <div className="flex flex-col h-screen bg-bg-deep overflow-hidden pb-14 md:pb-0">
      {/* Top nav */}
      <AppHeader />

      {/* Three-panel body — sidebars collapse on mobile */}
      <div className="flex flex-1 overflow-hidden min-h-0">
        {/* Left: Character & Quests — desktop only */}
        <div className="hidden md:flex">
          <CharacterSidebar />
        </div>

        {/* Center: Chat */}
        <main className="flex-1 flex flex-col min-w-0 md:border-x md:border-border-dim">
          <ChatPanel />
        </main>

        {/* Right: Materials / Mastery / Leaderboard — desktop only */}
        <div className="hidden md:flex">
          <MaterialsSidebar />
        </div>
      </div>

      {/* Bottom: XP progress bar — desktop only */}
      <div className="hidden md:block">
        <XpProgressBar current={2340} levelMax={3000} level={12} />
      </div>

      {/* Mobile bottom tab navigation */}
      <BottomNav />

      {/* Level-up overlay — shown on mount for demo; wire to gaming events in production */}
      <LevelUpBanner newLevel={13} />
    </div>
  )
}
