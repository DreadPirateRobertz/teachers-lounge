'use client'

/**
 * QuestionCard.tsx
 *
 * Displays a single multiple-choice question during a boss battle.
 * Each option renders as a tappable button; once selected the correct
 * answer is revealed and further input is locked until the parent
 * transitions to the attack/resolve phase.
 */

/** A single answer option received from the quiz API. */
export interface QuizOption {
  key: string
  text: string
}

/** A question received from the quiz API (correct_key is never sent). */
export interface BattleQuestion {
  id: string
  question: string
  options: QuizOption[]
  difficulty: number
  topic: string
  explanation?: string
  xp_reward: number
}

/** Props for QuestionCard. */
export interface QuestionCardProps {
  question: BattleQuestion
  /** Null while player hasn't answered yet. */
  chosenKey: string | null
  /** Null until the answer has been revealed. */
  correctKey: string | null
  onAnswer: (key: string) => void
  disabled: boolean
}

/**
 * QuestionCard renders the question text and MCQ option buttons.
 *
 * Before answering: all options are interactive.
 * After answering: correct option highlighted green, chosen-wrong highlighted
 * red, other options dimmed. Interaction is locked (disabled prop or
 * chosenKey is set).
 */
export default function QuestionCard({
  question,
  chosenKey,
  correctKey,
  onAnswer,
  disabled,
}: QuestionCardProps) {
  const answered = chosenKey !== null

  return (
    <div className="flex flex-col gap-4 w-full max-w-md">
      {/* Question text */}
      <div
        className="rounded-xl border border-neon-blue/30 bg-bg-card px-5 py-4
                   text-sm font-mono text-text-bright leading-relaxed"
      >
        <span className="text-xs text-neon-blue uppercase tracking-widest mr-2 opacity-60">Q</span>
        {question.question}
      </div>

      {/* Options */}
      <div className="flex flex-col gap-2">
        {question.options.map((opt) => {
          const isChosen = opt.key === chosenKey
          const isCorrect = opt.key === correctKey

          // Derive visual state
          let borderColor = '#334466'
          let bgColor = 'transparent'
          let textColor = '#aabbcc'
          let glowStyle = ''

          if (answered && isCorrect) {
            borderColor = '#00ff88'
            bgColor = '#00ff8820'
            textColor = '#00ff88'
            glowStyle = '0 0 10px #00ff8844'
          } else if (answered && isChosen && !isCorrect) {
            borderColor = '#ff3366'
            bgColor = '#ff336618'
            textColor = '#ff3366'
          } else if (!answered) {
            // Hover affordance handled by CSS classes
            borderColor = '#334466'
            textColor = '#ccd0d8'
          }

          return (
            <button
              key={opt.key}
              onClick={() => !answered && !disabled && onAnswer(opt.key)}
              disabled={disabled || answered}
              aria-label={`Option ${opt.key}: ${opt.text}`}
              className="w-full text-left px-4 py-3 rounded-xl border font-mono text-sm
                         transition-all duration-200 min-h-[44px]
                         not-disabled:hover:bg-neon-blue/10 not-disabled:active:scale-[0.98]
                         disabled:cursor-not-allowed touch-manipulation"
              style={{
                borderColor,
                backgroundColor: bgColor,
                color: textColor,
                boxShadow: glowStyle,
                touchAction: 'manipulation',
              }}
            >
              <span className="font-bold mr-2 opacity-70">{opt.key}.</span>
              {opt.text}
            </button>
          )
        })}
      </div>

      {/* Difficulty indicator */}
      <div className="flex items-center gap-1 justify-end">
        {Array.from({ length: 5 }).map((_, i) => (
          <span
            key={i}
            className="w-1.5 h-1.5 rounded-full"
            style={{
              backgroundColor: i < question.difficulty ? '#4488ff' : '#223344',
            }}
          />
        ))}
        <span className="text-[10px] font-mono text-text-dim ml-1">
          difficulty {question.difficulty}
        </span>
      </div>
    </div>
  )
}
