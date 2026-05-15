'use client'

/**
 * Felder-Silverman learning-style dials.
 *
 * Each axis is a signed score in roughly [-1.0, 1.0]; sign indicates the
 * dominant pole, magnitude indicates strength. The frontend uses the
 * dominant axis to label the student's overall style.
 */
export interface FelderDials {
  active_reflective: number
  sensing_intuitive: number
  visual_verbal: number
  sequential_global: number
}

/** Display metadata for a single learning-style label. */
interface StyleLabel {
  /** Short label shown in the badge (e.g. "Visual"). */
  label: string
  /** Single emoji icon. */
  icon: string
  /** Tailwind color classes for badge tint. */
  color: string
}

/** A balanced (no dominant axis) profile. */
const BALANCED: StyleLabel = {
  label: 'Balanced',
  icon: '⚖️',
  color: 'text-text-dim border-border-mid bg-bg-card',
}

/** Threshold below which a dial is considered balanced (no dominant pole). */
const DOMINANCE_THRESHOLD = 0.2

/**
 * Pick the dominant style label from a Felder-Silverman dial set.
 *
 * Returns the axis with the largest absolute score above {@link DOMINANCE_THRESHOLD}.
 * If no axis crosses the threshold, returns a "Balanced" label. If `dials` is
 * `null`, also returns "Balanced" so the badge can render before profile load.
 *
 * Mapping (negative → first label, positive → second label):
 * - active_reflective: Active 🤸 / Reflective 🧘
 * - sensing_intuitive: Sensing 🔬 / Intuitive 💡
 * - visual_verbal: Visual 👁️ / Auditory 🎧
 * - sequential_global: Sequential 📋 / Global 🌐
 *
 * @param dials - Current Felder-Silverman scores, or `null` if not yet loaded.
 * @returns The {@link StyleLabel} to display.
 */
export function pickStyleLabel(dials: FelderDials | null): StyleLabel {
  if (!dials) return BALANCED

  const candidates: Array<{ score: number; neg: StyleLabel; pos: StyleLabel }> = [
    {
      score: dials.visual_verbal,
      neg: {
        label: 'Visual',
        icon: '👁️',
        color: 'text-neon-blue border-neon-blue/40 bg-neon-blue/10',
      },
      pos: {
        label: 'Auditory',
        icon: '🎧',
        color: 'text-neon-pink border-neon-pink/40 bg-neon-pink/10',
      },
    },
    {
      score: dials.active_reflective,
      neg: {
        label: 'Active',
        icon: '🤸',
        color: 'text-neon-green border-neon-green/40 bg-neon-green/10',
      },
      pos: {
        label: 'Reflective',
        icon: '🧘',
        color: 'text-neon-blue border-neon-blue/40 bg-neon-blue/10',
      },
    },
    {
      score: dials.sensing_intuitive,
      neg: {
        label: 'Sensing',
        icon: '🔬',
        color: 'text-neon-gold border-neon-gold/40 bg-neon-gold/10',
      },
      pos: {
        label: 'Intuitive',
        icon: '💡',
        color: 'text-neon-gold border-neon-gold/40 bg-neon-gold/10',
      },
    },
    {
      score: dials.sequential_global,
      neg: {
        label: 'Sequential',
        icon: '📋',
        color: 'text-text-bright border-border-mid bg-bg-card',
      },
      pos: { label: 'Global', icon: '🌐', color: 'text-text-bright border-border-mid bg-bg-card' },
    },
  ]

  let best: { mag: number; label: StyleLabel } | null = null
  for (const c of candidates) {
    const mag = Math.abs(c.score)
    if (mag < DOMINANCE_THRESHOLD) continue
    if (!best || mag > best.mag) {
      best = { mag, label: c.score < 0 ? c.neg : c.pos }
    }
  }
  return best?.label ?? BALANCED
}

interface Props {
  /** Current Felder-Silverman dial scores; pass `null` while loading. */
  dials: FelderDials | null
}

/**
 * Compact badge that displays the student's currently detected learning
 * style. Used in the chat header so the student can see which mode the
 * tutor is optimizing responses for.
 */
export default function LearningStyleBadge({ dials }: Props) {
  const style = pickStyleLabel(dials)
  return (
    <span
      data-testid="learning-style-badge"
      aria-label={`Learning style: ${style.label}`}
      className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full border text-[10px] font-mono font-bold tracking-wider uppercase ${style.color}`}
    >
      <span aria-hidden>{style.icon}</span>
      <span>{style.label}</span>
    </span>
  )
}
