'use client'

/**
 * BossHUD.tsx
 *
 * Heads-up display for the boss battle: shows boss and player HP bars,
 * the current turn number, power-up buttons, and the latest taunt.
 */

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
}

/** All power-ups available in a boss fight. */
export const POWER_UPS: PowerUp[] = [
  { type: 'double_damage', label: 'Double DMG', icon: '⚔️', gemCost: 3 },
  { type: 'shield', label: 'Shield', icon: '🛡️', gemCost: 2 },
  { type: 'heal', label: 'Heal', icon: '💊', gemCost: 2 },
  { type: 'critical', label: 'Critical', icon: '💥', gemCost: 5 },
]

/**
 * HPBar renders a labeled health bar with a neon fill.
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
  const pct = maxHP > 0 ? Math.max(0, Math.min(100, Math.round((hp / maxHP) * 100))) : 0
  return (
    <div className="w-full">
      <div className="flex justify-between items-center mb-1">
        <span className="text-xs font-mono text-text-dim">{label}</span>
        <span className="text-xs font-mono text-text-bright">
          {hp} / {maxHP}
        </span>
      </div>
      <div className="h-3 w-full bg-bg-input rounded-full overflow-hidden border border-border-dim">
        <div
          className="h-full rounded-full transition-all duration-300"
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
