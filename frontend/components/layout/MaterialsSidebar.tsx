'use client'

import React, { useState } from 'react'
import MaterialUpload, { type UploadedMaterial } from '@/components/materials/MaterialUpload'
import MaterialLibrary from '@/components/materials/MaterialLibrary'

// Mock data — will be replaced by Gaming Service / Search Service API calls
const MASTERY_TOPICS = [
  { name: 'Atomic Structure', score: 0.88 },
  { name: 'Chemical Bonding', score: 0.71 },
  { name: 'Nomenclature', score: 0.65 },
  { name: 'Stereochemistry', score: 0.42 },
  { name: 'Organic Reactions', score: 0.29 },
]

const LEADERBOARD = [
  { rank: 1, name: 'MoleMaster', xp: 4820, isRival: true },
  { rank: 2, name: 'ChemWizard', xp: 2340, isMe: true },
  { rank: 3, name: 'BondBreaker', xp: 2100, isRival: true },
  { rank: 4, name: 'NovaStar', xp: 1950, isRival: true },
  { rank: 5, name: 'ReactKing', xp: 1780, isRival: true },
]

const POWERUPS = [
  { icon: '🧠', name: 'Hint', desc: 'Remove wrong answer', cost: 50, owned: 3 },
  { icon: '🛡️', name: 'Shield', desc: 'Block one hit', cost: 75, owned: 1 },
  { icon: '⚡', name: '2× Damage', desc: 'Double XP next answer', cost: 100, owned: 0 },
  { icon: '⏰', name: 'Time+', desc: '+15 sec to timer', cost: 60, owned: 2 },
]

// Phase 1 placeholder course ID — replaced by real course context in Phase 2
const DEFAULT_COURSE_ID = '00000000-0000-0000-0000-000000000001'

type Tab = 'mastery' | 'rankings' | 'powerups' | 'materials'

const TABS: { id: Tab; label: string }[] = [
  { id: 'mastery', label: 'Mastery' },
  { id: 'rankings', label: 'Rankings' },
  { id: 'powerups', label: 'Power-ups' },
  { id: 'materials', label: 'Materials' },
]

function masteryColor(score: number) {
  if (score >= 0.9) return 'bg-neon-gold'
  if (score >= 0.7) return 'bg-neon-green'
  if (score >= 0.4) return 'bg-yellow-400'
  return 'bg-red-500'
}

function masteryLabel(score: number) {
  if (score >= 0.9) return 'text-neon-gold'
  if (score >= 0.7) return 'text-neon-green'
  if (score >= 0.4) return 'text-yellow-400'
  return 'text-red-400'
}

export default function MaterialsSidebar() {
  const [activeTab, setActiveTab] = useState<Tab>('mastery')
  const [materials, setMaterials] = useState<UploadedMaterial[]>([])

  function handleUploadComplete(result: UploadedMaterial) {
    setMaterials((prev: UploadedMaterial[]) => [result, ...prev])
  }

  return (
    <aside className="w-[280px] flex-shrink-0 flex flex-col bg-bg-panel overflow-hidden">
      {/* Tab bar */}
      <div className="flex border-b border-border-dim flex-shrink-0 overflow-x-auto">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`flex-1 text-center py-2 text-[11px] font-medium border-b-2 transition-colors whitespace-nowrap ${
              activeTab === tab.id
                ? 'text-neon-blue border-neon-blue'
                : 'text-text-dim border-transparent hover:text-text-base'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Panel content */}
      <div className="flex-1 overflow-y-auto p-3">
        {activeTab === 'mastery' && <MasteryPanel />}
        {activeTab === 'rankings' && <RankingsPanel />}
        {activeTab === 'powerups' && <PowerupsPanel />}
        {activeTab === 'materials' && (
          <MaterialsPanel
            materials={materials}
            onUploadComplete={handleUploadComplete}
          />
        )}
      </div>
    </aside>
  )
}

function MasteryPanel() {
  return (
    <>
      <SectionLabel>Topic Mastery</SectionLabel>
      <div className="flex flex-col gap-2">
        {MASTERY_TOPICS.map((t) => (
          <div key={t.name}>
            <div className="flex justify-between text-xs mb-1">
              <span className="text-text-base truncate pr-2">{t.name}</span>
              <span className={`font-mono font-bold flex-shrink-0 ${masteryLabel(t.score)}`}>
                {Math.round(t.score * 100)}%
              </span>
            </div>
            <div className="h-1.5 bg-border-dim rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all ${masteryColor(t.score)}`}
                style={{ width: `${t.score * 100}%` }}
              />
            </div>
          </div>
        ))}
      </div>
    </>
  )
}

function RankingsPanel() {
  return (
    <>
      <SectionLabel>Rankings</SectionLabel>
      <div className="flex flex-col gap-1">
        {LEADERBOARD.map((entry) => (
          <div
            key={entry.rank}
            className={`flex items-center gap-2 px-2 py-1.5 rounded text-xs ${
              entry.isMe
                ? 'bg-neon-blue/10 border border-neon-blue/30'
                : 'bg-bg-card border border-border-dim'
            }`}
          >
            <span className={`font-mono font-bold w-4 flex-shrink-0 ${entry.rank === 1 ? 'text-neon-gold' : 'text-text-dim'}`}>
              {entry.rank}
            </span>
            <span className={`flex-1 truncate ${entry.isMe ? 'text-neon-blue font-medium' : 'text-text-base'}`}>
              {entry.name}
              {entry.isMe && ' (you)'}
              {entry.isRival && !entry.isMe && (
                <span className="text-text-dim ml-1 text-[10px]">rival</span>
              )}
            </span>
            <span className="font-mono text-[10px] text-text-dim flex-shrink-0">
              {entry.xp.toLocaleString()}
            </span>
          </div>
        ))}
      </div>
    </>
  )
}

function PowerupsPanel() {
  return (
    <>
      <div className="flex items-center justify-between mb-3">
        <SectionLabel>Power-ups</SectionLabel>
        <div className="flex items-center gap-1 text-[10px] text-neon-pink font-mono mb-3">
          <span>💎</span>
          <span>450</span>
        </div>
      </div>
      <div className="flex flex-col gap-2">
        {POWERUPS.map((p) => (
          <PowerUpItem key={p.name} powerup={p} />
        ))}
      </div>
    </>
  )
}

function PowerUpItem({ powerup }: { powerup: typeof POWERUPS[0] }) {
  const hasOwned = powerup.owned > 0
  return (
    <div
      className={`rounded-lg border p-2 transition-colors ${
        hasOwned
          ? 'bg-bg-card border-border-mid hover:border-neon-green/40'
          : 'bg-bg-card border-border-dim opacity-70'
      }`}
      style={hasOwned ? { boxShadow: 'inset 0 0 12px #00ff8808' } : undefined}
    >
      <div className="flex items-start gap-2">
        {/* Icon with owned indicator */}
        <div className="relative flex-shrink-0">
          <span className="text-xl leading-none">{powerup.icon}</span>
          {hasOwned && (
            <span className="absolute -top-1 -right-1 w-3.5 h-3.5 rounded-full bg-neon-green text-bg-deep font-mono font-black text-[8px] flex items-center justify-center">
              {powerup.owned}
            </span>
          )}
        </div>

        {/* Name + desc */}
        <div className="flex-1 min-w-0">
          <div className={`text-xs font-medium ${hasOwned ? 'text-text-bright' : 'text-text-base'}`}>
            {powerup.name}
          </div>
          <div className="text-[10px] text-text-dim leading-tight">{powerup.desc}</div>
        </div>

        {/* Cost / use button */}
        <div className="flex-shrink-0 flex flex-col items-end gap-1">
          {hasOwned ? (
            <button className="text-[10px] font-mono font-bold text-neon-green border border-neon-green/30 rounded px-1.5 py-0.5 hover:bg-neon-green/10 transition-colors">
              USE
            </button>
          ) : (
            <button className="text-[10px] font-mono font-bold text-neon-pink border border-neon-pink/30 rounded px-1.5 py-0.5 hover:bg-neon-pink/10 transition-colors">
              BUY
            </button>
          )}
          <span className="font-mono text-[9px] text-text-dim">💎{powerup.cost}</span>
        </div>
      </div>
    </div>
  )
}

interface MaterialsPanelProps {
  materials: UploadedMaterial[]
  onUploadComplete: (result: UploadedMaterial) => void
}

function MaterialsPanel({ materials, onUploadComplete }: MaterialsPanelProps) {
  return (
    <>
      <SectionLabel>Upload Material</SectionLabel>
      <MaterialUpload courseId={DEFAULT_COURSE_ID} onUploadComplete={onUploadComplete} />

      <div className="mt-4">
        <SectionLabel>Library ({materials.length})</SectionLabel>
        <MaterialLibrary materials={materials} />
      </div>
    </>
  )
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <div className="text-[10px] font-bold text-text-dim uppercase tracking-wider mb-3">
      {children}
    </div>
  )
}
