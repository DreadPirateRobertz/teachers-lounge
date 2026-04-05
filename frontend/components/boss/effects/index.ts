/**
 * Barrel export for all Weird Science boss battle VFX hooks and components.
 */

export { useParticleBurst } from './useParticleBurst'
export type {
  Particle,
  BurstType,
  BurstOrigin as ParticleBurstOrigin,
  UseParticleBurstReturn,
} from './useParticleBurst'

export { default as ParticleBurst } from './ParticleBurst'
export type { ParticleBurstHandle, ParticleBurstProps } from './ParticleBurst'

export { useScreenShake } from './useScreenShake'
export type { UseScreenShakeReturn } from './useScreenShake'

export { useMoleculeMorph } from './useMoleculeMorph'
export type { UseMoleculeMorphReturn } from './useMoleculeMorph'

export { useDeathDissolution } from './useDeathDissolution'
export type { UseDeathDissolutionReturn } from './useDeathDissolution'

export { useComboStreak } from './useComboStreak'
export type { StreakLevel, UseComboStreakReturn } from './useComboStreak'

export { default as ComboStreakBadge } from './ComboStreakBadge'
export type { ComboStreakBadgeProps } from './ComboStreakBadge'

export { default as BattleEffects } from './BattleEffects'
export type { BattleEffectsHandle, BattleEffectsProps, BurstOrigin } from './BattleEffects'
