'use client'

/**
 * BossHUD.tsx
 *
 * Heads-up display for the boss battle: shows boss and player HP bars,
 * the current turn number, power-up buttons, and the latest taunt.
 */

import { useEffect, useRef, useState } from 'react'
import { type BossVisualDef } from './BossCharacterLibrary'

/** A power-up option the player can spend gems to activate. */
export interface PowerUp {
  type: 'double_damage' | 'shield' | 'heal' | 'critical'
  label: string
  icon: string
  gemCost: number
}

/** Props for the BossHUD component. */
export interface BossHUDProps {
  boss: BossVisualDef
  bossHP: number
  bossMaxHP: number
  playerHP: number
  playerMaxHP: number
  turn: number
  gems: number
  taunt: string | null
  onPowerUpAction: (type: PowerUp['type']) => void
  disabled: boolean
  /**
   * Consecutive correct answers in the current streak. When ≥ 2, a combo
   * badge is rendered above the HP bars. Optional — omit to hide entirely.
   */
  comboCount?: number
}

/** All power-ups available in a boss fight. */
export const POWER_UPS: PowerUp[] = [
  { type: 'double_damage', label: 'Double DMG', icon: '⚔️', gemCost: 3 },
  { type: 'shield', label: 'Shield', icon: '🛡️', gemCost: 2 },
  { type: 'heal', label: 'Heal', icon: '💊', gemCost: 2 },
  { type: 'critical', label: 'Critical', icon: '💥', gemCost: 5 },
]

/** Duration in ms over which the displayed HP number tweens to a new target. */
const HP_TWEEN_MS = 600

/**
 * useTweenedNumber animates a numeric value toward `target` over `durationMs`,
 * returning the rounded interpolated value each frame. Cancels in-flight
 * tweens when `target` changes mid-animation. SSR-safe — falls straight to
 * `target` when `requestAnimationFrame` is unavailable.
 */
export function useTweenedNumber(target: number, durationMs: number = HP_TWEEN_MS): number {
  const [display, setDisplay] = useState(target)
  const fromRef = useRef(target)

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.requestAnimationFrame !== 'function') {
      setDisplay(target)
      return
    }
    const from = fromRef.current
    if (from === target) return
    const start = performance.now()
    let rafId = 0

    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / durationMs)
      // ease-out cubic — fast start, gentle settle
      const eased = 1 - Math.pow(1 - t, 3)
      const value = Math.round(from + (target - from) * eased)
      setDisplay(value)
      if (t < 1) {
        rafId = window.requestAnimationFrame(tick)
      } else {
        fromRef.current = target
      }
    }
    rafId = window.requestAnimationFrame(tick)
    return () => {
      if (typeof window.cancelAnimationFrame === 'function') {
        window.cancelAnimationFrame(rafId)
      }
    }
  }, [target, durationMs])

  return display
}

/**
 * HPBar renders a labeled health bar with a neon fill. The numeric readout
 * tweens between values over ~600 ms (ease-out) and the fill width animates
 * via CSS over the same window so visible changes feel weighty rather than
 * snapping instantly.
 */
function HPBar({
  label,
  hp,
  maxHP,
  color,
}: {
  label: string
  hp: number
  maxHP: number
  color: string
}) {
  const tweenedHp = useTweenedNumber(hp)
  const pct = maxHP > 0 ? Math.max(0, Math.min(100, Math.round((tweenedHp / maxHP) * 100))) : 0
  return (
    <div className="w-full">
      <div className="flex justify-between items-center mb-1">
        <span className="text-xs font-mono text-text-dim">{label}</span>
        <span className="text-xs font-mono text-text-bright" data-testid={`hp-readout-${label}`}>
          {tweenedHp} / {maxHP}
        </span>
      </div>
      <div className="h-3 w-full bg-bg-input rounded-full overflow-hidden border border-border-dim">
        <div
          className="h-full rounded-full transition-all duration-700 ease-out"
          style={{ width: `${pct}%`, backgroundColor: color, boxShadow: `0 0 6px ${color}88` }}
        />
      </div>
    </div>
  )
}

/**
 * BossHUD displays all real-time battle information and player controls.
 */
export default function BossHUD({
  boss,
  bossHP,
  bossMaxHP,
  playerHP,
  playerMaxHP,
  turn,
  gems,
  taunt,
  onPowerUpAction,
  disabled,
  comboCount,
}: BossHUDProps) {
  return (
    <div className="flex flex-col gap-3 w-full max-w-md px-4">
      {/* Boss identity */}
      <div className="flex items-center gap-2">
        <span
          className="text-sm font-mono font-bold tracking-wider"
          style={{ color: boss.primaryColor, textShadow: `0 0 8px ${boss.primaryColor}` }}
        >
          {boss.name}
        </span>
        <span className="text-xs text-text-dim font-mono">Tier {boss.tier}</span>
        <span className="ml-auto text-xs text-text-dim font-mono">Turn {turn}</span>
      </div>

      {/* Combo streak — only when ≥ 2 consecutive correct answers */}
      {comboCount !== undefined && comboCount >= 2 && (
        <div
          data-testid="combo-badge"
          aria-label={`Combo streak: ${comboCount}`}
          className="self-end text-xs font-mono font-bold px-2 py-1 rounded-md border border-neon-gold/50 bg-neon-gold/10 text-neon-gold animate-pulse-slow"
          style={{ textShadow: '0 0 6px #ffd700aa' }}
        >
          ⚡ {comboCount}× COMBO
        </div>
      )}

      {/* Boss HP */}
      <HPBar label="BOSS HP" hp={bossHP} maxHP={bossMaxHP} color={boss.primaryColor} />

      {/* Player HP */}
      <HPBar label="YOUR HP" hp={playerHP} maxHP={playerMaxHP} color="#00aaff" />

      {/* Taunt */}
      {taunt && (
        <div
          className="text-xs font-mono italic text-center py-2 px-3 rounded-lg border"
          style={{
            color: boss.primaryColor,
            borderColor: `${boss.primaryColor}44`,
            backgroundColor: `${boss.primaryColor}0a`,
          }}
        >
          &ldquo;{taunt}&rdquo;
        </div>
      )}

      {/* Power-ups */}
      <div className="flex gap-2 flex-wrap">
        {POWER_UPS.map((pu) => {
          const canAfford = gems >= pu.gemCost
          return (
            <button
              key={pu.type}
              onClick={() => onPowerUpAction(pu.type)}
              disabled={disabled || !canAfford}
              className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-mono border transition-colors
                disabled:opacity-40 disabled:cursor-not-allowed
                enabled:hover:bg-neon-blue/10 enabled:active:scale-95"
              style={{
                borderColor: canAfford ? '#00aaff44' : '#333355',
                color: canAfford ? '#00aaff' : '#6666aa',
              }}
              aria-label={`${pu.label} (${pu.gemCost} gems)`}
            >
              <span>{pu.icon}</span>
              <span>{pu.label}</span>
              <span className="text-neon-gold">{pu.gemCost}💎</span>
            </button>
          )
        })}
      </div>

      {/* Gem count */}
      <div className="text-xs font-mono text-text-dim text-right">
        Gems: <span className="text-neon-gold font-bold">{gems}</span>
      </div>
    </div>
  )
}
