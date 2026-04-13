'use client'

import { useEffect, useRef, useState } from 'react'
import MaterialUpload, { UploadedMaterial } from '@/components/materials/MaterialUpload'
import MaterialLibrary from '@/components/materials/MaterialLibrary'
import LeaderboardPanel from './LeaderboardPanel'
import ErrorBoundary from '@/components/ErrorBoundary'

/** Statuses for which polling should stop. */
const TERMINAL_STATUSES: ReadonlySet<UploadedMaterial['status']> = new Set(['complete', 'failed'])

/** Polling interval in ms — mirrors useMaterialStatus constant. */
const POLL_INTERVAL_MS = 3_000

const MASTERY_TOPICS = [
  { name: 'Atomic Structure', score: 0.88 },
  { name: 'Chemical Bonding', score: 0.71 },
  { name: 'Nomenclature', score: 0.65 },
  { name: 'Stereochemistry', score: 0.42 },
  { name: 'Organic Reactions', score: 0.29 },
]

const POWERUPS = [
  { icon: '🧠', name: 'Hint', desc: 'Remove wrong answer', cost: 50, owned: 3 },
  { icon: '🛡️', name: 'Shield', desc: 'Block one hit', cost: 75, owned: 1 },
  { icon: '⚡', name: '2x Damage', desc: 'Double XP next answer', cost: 100, owned: 0 },
  { icon: '⏰', name: 'Time+', desc: '+15 sec to timer', cost: 60, owned: 2 },
]

type Tab = 'mastery' | 'rankings' | 'powerups' | 'materials'

interface Props {
  /** Course UUID to associate uploads with.  Defaults to a dev placeholder. */
  courseId?: string
}

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

/**
 * Right-hand sidebar with four tabs: Mastery, Rankings, Power-ups, and Materials.
 *
 * The Materials tab provides a full upload + library experience with built-in
 * status polling so callers have no extra responsibilities.
 *
 * @param courseId - Optional course UUID for uploads; defaults to a dev placeholder.
 */
export default function MaterialsSidebar({ courseId }: Props = {}) {
  const [activeTab, setActiveTab] = useState<Tab>('mastery')

  return (
    <aside className="w-[280px] flex-shrink-0 flex flex-col bg-bg-panel overflow-hidden">
      {/* Tab bar */}
      <div className="flex border-b border-border-dim flex-shrink-0">
        <SidebarTab
          label="Mastery"
          active={activeTab === 'mastery'}
          onClick={() => setActiveTab('mastery')}
        />
        <SidebarTab
          label="Rankings"
          active={activeTab === 'rankings'}
          onClick={() => setActiveTab('rankings')}
        />
        <SidebarTab
          label="Power-ups"
          active={activeTab === 'powerups'}
          onClick={() => setActiveTab('powerups')}
        />
        <SidebarTab
          label="Materials"
          active={activeTab === 'materials'}
          onClick={() => setActiveTab('materials')}
        />
      </div>

      {/* Tab content */}
      <div className="flex-1 overflow-y-auto p-3">
        {activeTab === 'mastery' && <MasteryPanel />}
        {activeTab === 'rankings' && (
          <ErrorBoundary componentName="Leaderboard">
            <LeaderboardPanel />
          </ErrorBoundary>
        )}
        {activeTab === 'powerups' && <PowerupsPanel />}
        {activeTab === 'materials' && (
          <MaterialsPanel courseId={courseId ?? 'dev-placeholder-course-id'} />
        )}
      </div>
    </aside>
  )
}

// ---------------------------------------------------------------------------
// MaterialsPanel
// ---------------------------------------------------------------------------

/**
 * Upload + library panel with built-in status polling.
 *
 * Maintains the local list of uploaded materials and runs a single polling
 * interval scoped to the panel's lifetime.  The interval reads from a ref on
 * each tick so it never needs to be re-created when state changes.
 *
 * @param courseId - Course UUID forwarded to MaterialUpload.
 */
function MaterialsPanel({ courseId }: { courseId: string }) {
  const [materials, setMaterials] = useState<UploadedMaterial[]>([])
  const materialsRef = useRef<UploadedMaterial[]>([])
  materialsRef.current = materials

  /**
   * Prepend a newly-uploaded material to the list.
   *
   * @param m - Material metadata returned by the upload API.
   */
  function handleUploadComplete(m: UploadedMaterial) {
    setMaterials((prev) => [m, ...prev])
  }

  useEffect(() => {
    const id = setInterval(async () => {
      const toCheck = materialsRef.current.filter((m) => !TERMINAL_STATUSES.has(m.status))
      if (toCheck.length === 0) return

      const results = await Promise.allSettled(
        toCheck.map(async (m) => {
          const res = await fetch(`/api/materials/${m.materialId}/status`)
          if (!res.ok) return null
          const data = (await res.json()) as { status: UploadedMaterial['status'] }
          return { materialId: m.materialId, status: data.status }
        }),
      )

      const updates = new Map<string, UploadedMaterial['status']>()
      for (const r of results) {
        if (r.status === 'fulfilled' && r.value !== null) {
          updates.set(r.value.materialId, r.value.status)
        }
      }

      if (updates.size > 0) {
        setMaterials((prev) =>
          prev.map((m) =>
            updates.has(m.materialId) ? { ...m, status: updates.get(m.materialId)! } : m,
          ),
        )
      }
    }, POLL_INTERVAL_MS)

    return () => clearInterval(id)
  }, [])

  return (
    <div className="flex flex-col gap-3">
      <MaterialUpload courseId={courseId} onUploadComplete={handleUploadComplete} />
      {materials.length > 0 && (
        <div className="border-t border-border-dim pt-2">
          <div className="text-[10px] font-bold text-text-dim uppercase tracking-wider mb-2">
            Your Materials
          </div>
          <MaterialLibrary materials={materials} />
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Sub-panels
// ---------------------------------------------------------------------------

function MasteryPanel() {
  return (
    <>
      <div className="text-[10px] font-bold text-text-dim uppercase tracking-wider mb-3">
        Topic Mastery
      </div>
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

function PowerupsPanel() {
  return (
    <>
      <div className="text-[10px] font-bold text-text-dim uppercase tracking-wider mb-3">
        Power-ups
      </div>
      <div className="flex flex-col gap-1.5">
        {POWERUPS.map((p) => (
          <div
            key={p.name}
            className="flex items-center gap-2 bg-bg-card border border-border-dim rounded p-2"
          >
            <span className="text-lg leading-none">{p.icon}</span>
            <div className="flex-1 min-w-0">
              <div className="text-xs text-text-base font-medium">{p.name}</div>
              <div className="text-[10px] text-text-dim">{p.desc}</div>
            </div>
            <div className="flex flex-col items-end flex-shrink-0">
              <span className="font-mono text-[10px] text-neon-pink">💎{p.cost}</span>
              <span className="font-mono text-[10px] text-text-dim">x{p.owned}</span>
            </div>
          </div>
        ))}
      </div>
    </>
  )
}

function SidebarTab({
  label,
  active,
  onClick,
}: {
  label: string
  active: boolean
  onClick: () => void
}) {
  return (
    <button
      onClick={onClick}
      className={`flex-1 text-center py-2 text-[11px] font-medium border-b-2 transition-colors ${
        active
          ? 'text-neon-blue border-neon-blue'
          : 'text-text-dim border-transparent hover:text-text-base'
      }`}
    >
      {label}
    </button>
  )
}
