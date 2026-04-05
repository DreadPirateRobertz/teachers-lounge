/**
 * BossCharacterLibrary.ts
 *
 * Defines all six chemistry boss characters with their visual configurations,
 * animation parameters, and taunt pools. This is pure data — no Three.js imports.
 * The BossScene component consumes these definitions to build procedural geometry.
 *
 * Visual identity: Weird Science aesthetic — Neon Arcade palette on #0a0a1a,
 * glowing molecular structures, real-time morphing meshes.
 */

/** Animation state that drives which Three.js loop runs. */
export type AnimationState = 'idle' | 'attack' | 'damage' | 'death'

/** Geometry type that tells the scene builder which mesh to construct. */
export type BossGeometry =
  | 'atom'
  | 'bonder'
  | 'name_lord'
  | 'stereochemist'
  | 'reactor'
  | 'final_boss'

/** Parameters for a single animation state. */
export interface AnimationParams {
  /** Human-readable description of the animation. */
  description: string
  /** Duration of one full animation cycle in seconds. */
  cycleSec: number
  /** Multiplier applied to primary rotation speed. */
  rotationSpeed: number
  /** Amplitude of any oscillation (0 = none). */
  oscillationAmp: number
  /** Particle burst count (0 = no particles). */
  particleCount: number
  /** Whether to flash the mesh with the accent color. */
  colorFlash: boolean
}

/** Full animation config for a boss, covering all four states. */
export interface BossAnimationConfig {
  idle: AnimationParams
  attack: AnimationParams
  damage: AnimationParams
  death: AnimationParams
}

/** Complete visual and combat definition for a boss. */
export interface BossVisualDef {
  /** Machine-friendly ID matching the backend catalog. */
  id: string
  /** Display name. */
  name: string
  /** Chemistry topic this boss guards. */
  topic: string
  /** Difficulty tier 1–6. */
  tier: number
  /** Three.js scene geometry builder to use. */
  geometry: BossGeometry
  /** Dominant neon color (hex). */
  primaryColor: string
  /** Secondary color for glow and bond highlights (hex). */
  secondaryColor: string
  /** Accent used on hits, flashes, and death (hex). */
  accentColor: string
  /** Scale factor for the overall mesh. Tier 6 is largest. */
  scale: number
  /** Per-state animation parameters. */
  animations: BossAnimationConfig
  /** Lines displayed when the student answers wrong. */
  tauntPool: string[]
}

/**
 * BOSS_LIBRARY maps boss IDs to their full visual definitions.
 *
 * All six chemistry bosses are represented in chapter order (tier 1–6).
 * The final boss (tier 6) is only accessible after all chapter bosses are defeated.
 */
export const BOSS_LIBRARY: Readonly<Record<string, BossVisualDef>> = {
  the_atom: {
    id: 'the_atom',
    name: 'THE ATOM',
    topic: 'general_chemistry',
    tier: 1,
    geometry: 'atom',
    primaryColor: '#00aaff',
    secondaryColor: '#00ff88',
    accentColor: '#ffffff',
    scale: 1.0,
    animations: {
      idle: {
        description: 'Nucleus pulses; electron rings orbit at varying speeds',
        cycleSec: 3,
        rotationSpeed: 0.8,
        oscillationAmp: 0.1,
        particleCount: 0,
        colorFlash: false,
      },
      attack: {
        description: 'Electron beams fire outward from orbiting shells',
        cycleSec: 1.2,
        rotationSpeed: 3.0,
        oscillationAmp: 0.3,
        particleCount: 12,
        colorFlash: false,
      },
      damage: {
        description: 'Nucleus recoils; shells momentarily collapse inward',
        cycleSec: 0.4,
        rotationSpeed: 0.2,
        oscillationAmp: 0.5,
        particleCount: 6,
        colorFlash: true,
      },
      death: {
        description: 'Nucleus explodes; electron shells scatter as particles',
        cycleSec: 2.0,
        rotationSpeed: 0.0,
        oscillationAmp: 2.0,
        particleCount: 64,
        colorFlash: true,
      },
    },
    tauntPool: [
      'Your electrons are all over the place!',
      'Electron configuration: WRONG.',
      'Did you forget about quantum numbers?',
      'That answer had zero valence electrons of correctness.',
      'Back to the periodic table with you!',
    ],
  },

  the_bonder: {
    id: 'the_bonder',
    name: 'THE BONDER',
    topic: 'molecular_bonding',
    tier: 2,
    geometry: 'bonder',
    primaryColor: '#00ff88',
    secondaryColor: '#ffdc00',
    accentColor: '#00aaff',
    scale: 1.1,
    animations: {
      idle: {
        description: 'Two spheres connected by oscillating bond cylinders',
        cycleSec: 2.5,
        rotationSpeed: 0.5,
        oscillationAmp: 0.2,
        particleCount: 0,
        colorFlash: false,
      },
      attack: {
        description: 'Dual heads split apart then reform, firing bond energy',
        cycleSec: 1.0,
        rotationSpeed: 2.0,
        oscillationAmp: 1.5,
        particleCount: 20,
        colorFlash: false,
      },
      damage: {
        description: 'Bond cylinders deform; heads flash and recoil',
        cycleSec: 0.5,
        rotationSpeed: 0.3,
        oscillationAmp: 0.6,
        particleCount: 8,
        colorFlash: true,
      },
      death: {
        description: 'Bond shatters; both heads implode with energy burst',
        cycleSec: 2.2,
        rotationSpeed: 0.0,
        oscillationAmp: 3.0,
        particleCount: 80,
        colorFlash: true,
      },
    },
    tauntPool: [
      'Weak bonds break — just like that answer!',
      "That's not how covalent bonding works.",
      "Ionic or covalent? You clearly don't know.",
      'Your bond angles are catastrophically wrong.',
      'VSEPR theory weeps at your answer.',
    ],
  },

  name_lord: {
    id: 'name_lord',
    name: 'NAME LORD',
    topic: 'nomenclature',
    tier: 3,
    geometry: 'name_lord',
    primaryColor: '#ff00aa',
    secondaryColor: '#ffdc00',
    accentColor: '#ffffff',
    scale: 1.15,
    animations: {
      idle: {
        description: 'Icosahedron slowly morphing between molecular shapes',
        cycleSec: 4.0,
        rotationSpeed: 0.4,
        oscillationAmp: 0.15,
        particleCount: 0,
        colorFlash: false,
      },
      attack: {
        description: 'Rapid-fire shape shifts with cascading name labels',
        cycleSec: 0.8,
        rotationSpeed: 5.0,
        oscillationAmp: 0.5,
        particleCount: 30,
        colorFlash: false,
      },
      damage: {
        description: 'Geometry briefly collapses; surface flashes white',
        cycleSec: 0.4,
        rotationSpeed: 0.1,
        oscillationAmp: 0.8,
        particleCount: 10,
        colorFlash: true,
      },
      death: {
        description: 'Shatters into geometric fragments with label particles',
        cycleSec: 2.5,
        rotationSpeed: 0.0,
        oscillationAmp: 2.5,
        particleCount: 100,
        colorFlash: true,
      },
    },
    tauntPool: [
      'You dare mislabel compounds in MY presence?!',
      'That name is so wrong it broke the IUPAC handbook.',
      'Did you just call that a... never mind. WRONG.',
      'Names have POWER. Yours has none.',
      'I have been named. You have named nothing correctly.',
    ],
  },

  the_stereochemist: {
    id: 'the_stereochemist',
    name: 'THE STEREOCHEMIST',
    topic: 'stereochemistry',
    tier: 4,
    geometry: 'stereochemist',
    primaryColor: '#cc44ff',
    secondaryColor: '#00aaff',
    accentColor: '#ff00aa',
    scale: 1.2,
    animations: {
      idle: {
        description: 'Paired mirrored tetrahedra orbiting a central axis',
        cycleSec: 3.5,
        rotationSpeed: 0.6,
        oscillationAmp: 0.1,
        particleCount: 0,
        colorFlash: false,
      },
      attack: {
        description: 'Mirror clone advances while chirality flips mid-flight',
        cycleSec: 1.1,
        rotationSpeed: 2.5,
        oscillationAmp: 1.0,
        particleCount: 16,
        colorFlash: false,
      },
      damage: {
        description: 'Both tetrahedra flash and momentarily merge',
        cycleSec: 0.45,
        rotationSpeed: 0.2,
        oscillationAmp: 0.7,
        particleCount: 8,
        colorFlash: true,
      },
      death: {
        description: 'Mirror shatters; both enantiomers dissolve in spiral',
        cycleSec: 2.8,
        rotationSpeed: 0.0,
        oscillationAmp: 3.5,
        particleCount: 90,
        colorFlash: true,
      },
    },
    tauntPool: [
      'Mirror, mirror — and your knowledge is shattered!',
      'R or S? You chose... poorly.',
      'Enantiomers are not the same. Neither are your answers.',
      'That stereocenter is crying right now.',
      'Chiral. Unlike your reasoning, which has no handedness.',
    ],
  },

  the_reactor: {
    id: 'the_reactor',
    name: 'THE REACTOR',
    topic: 'organic_reactions',
    tier: 5,
    geometry: 'reactor',
    primaryColor: '#ff6600',
    secondaryColor: '#ffdc00',
    accentColor: '#ff0000',
    scale: 1.3,
    animations: {
      idle: {
        description: 'Churning torus vessel with swirling reaction particles',
        cycleSec: 2.0,
        rotationSpeed: 1.2,
        oscillationAmp: 0.05,
        particleCount: 24,
        colorFlash: false,
      },
      attack: {
        description: 'Chain reaction: particles burst outward in cascade',
        cycleSec: 0.9,
        rotationSpeed: 4.0,
        oscillationAmp: 0.8,
        particleCount: 40,
        colorFlash: false,
      },
      damage: {
        description: 'Vessel wall deforms; interior particles scatter',
        cycleSec: 0.5,
        rotationSpeed: 0.5,
        oscillationAmp: 1.0,
        particleCount: 15,
        colorFlash: true,
      },
      death: {
        description: 'Catastrophic explosion: torus fragments with fireball',
        cycleSec: 3.0,
        rotationSpeed: 0.0,
        oscillationAmp: 4.0,
        particleCount: 150,
        colorFlash: true,
      },
    },
    tauntPool: [
      'Your reaction mechanism is a catastrophic failure!',
      "SN1? SN2? You clearly don't know the difference.",
      "That's not a nucleophile, that's an embarrassment.",
      'Leaving groups are leaving. Just like your GPA.',
      'The reaction is exothermic. Your knowledge is endothermic.',
    ],
  },

  final_boss: {
    id: 'final_boss',
    name: 'FINAL BOSS',
    topic: 'course_final',
    tier: 6,
    geometry: 'final_boss',
    primaryColor: '#ff00aa',
    secondaryColor: '#00aaff',
    accentColor: '#ffdc00',
    scale: 1.6,
    animations: {
      idle: {
        description:
          'Composite entity: orbiting electrons, bonded arms, shifting shape, mirrored core, reactive aura',
        cycleSec: 5.0,
        rotationSpeed: 0.3,
        oscillationAmp: 0.2,
        particleCount: 16,
        colorFlash: false,
      },
      attack: {
        description: 'Multi-phase attack cycling through all chapter boss patterns',
        cycleSec: 1.5,
        rotationSpeed: 3.5,
        oscillationAmp: 1.2,
        particleCount: 60,
        colorFlash: false,
      },
      damage: {
        description: 'Outer shell cracks; inner structure pulses with pain',
        cycleSec: 0.6,
        rotationSpeed: 0.3,
        oscillationAmp: 0.9,
        particleCount: 20,
        colorFlash: true,
      },
      death: {
        description: 'Final dissolution: all component geometries shatter sequentially',
        cycleSec: 4.0,
        rotationSpeed: 0.0,
        oscillationAmp: 5.0,
        particleCount: 256,
        colorFlash: true,
      },
    },
    tauntPool: [
      'All my predecessors warned me. You are nothing.',
      "You've come so far only to fail now.",
      'I contain all of chemistry. You know a fraction.',
      'Every mistake you made brought you here.',
      'This is where the learning ends.',
      'I am every concept. I am every exam.',
      'They all fell. You will too.',
      'The course final claims another victim.',
    ],
  },
} as const

/**
 * getBossDef returns the visual definition for a boss ID.
 * Returns undefined if the boss is not in the library.
 */
export function getBossDef(id: string): BossVisualDef | undefined {
  return BOSS_LIBRARY[id]
}

/**
 * getBossesInOrder returns all boss definitions sorted by tier ascending.
 * Useful for rendering boss selection screens.
 */
export function getBossesInOrder(): BossVisualDef[] {
  return Object.values(BOSS_LIBRARY).sort((a, b) => a.tier - b.tier)
}

/**
 * getRandomTaunt returns a random taunt from a boss's taunt pool.
 */
export function getRandomTaunt(bossId: string): string {
  const def = getBossDef(bossId)
  if (!def || def.tauntPool.length === 0) return 'Wrong!'
  const idx = Math.floor(Math.random() * def.tauntPool.length)
  return def.tauntPool[idx]
}
