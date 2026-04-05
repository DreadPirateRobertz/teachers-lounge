/**
 * Boss character type definitions for the Weird Science battle UI.
 */

/** Unique identifier for each boss in the chemistry-themed catalog. */
export type BossId =
  | 'the_atom'
  | 'bonding_brothers'
  | 'name_lord'
  | 'the_stereochemist'
  | 'the_reactor'

/** Animation states for the boss character state machine. */
export type AnimationState = 'idle' | 'attack' | 'damage' | 'death'

/** Three.js geometry type identifier for each boss's visual representation. */
export type GeometryType = 'atom' | 'dumbbell' | 'grid' | 'tetrahedron' | 'flask'

/**
 * Full configuration for a single boss character, combining game data with
 * visual/rendering properties.
 */
export interface BossConfig {
  /** Machine-friendly identifier. */
  id: BossId
  /** Display name shown to the student. */
  name: string
  /** Boss tier (1–5); drives difficulty scaling. */
  tier: number
  /** Chemistry topic this boss guards. */
  topic: string
  /** Taunt message shown on a wrong answer. */
  taunt: string
  /** Primary hex color for the boss. */
  color: string
  /** Accent hex color for highlights and emissive glow. */
  accentColor: string
  /** Scale multiplier applied to the Three.js group. */
  scale: number
  /** Three.js geometry type used to build this boss's mesh. */
  geometryType: GeometryType
}

/**
 * Emitted when the boss animation transitions to a new state.
 */
export interface BossAnimationEvent {
  /** The new animation state. */
  state: AnimationState
  /** Unix timestamp (ms) of the transition. */
  timestamp: number
}
