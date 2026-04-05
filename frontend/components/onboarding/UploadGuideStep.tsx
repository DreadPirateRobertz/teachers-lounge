'use client'

/**
 * Wizard step explaining how to upload study materials.
 *
 * A visual walkthrough so users know how to get value from the platform
 * immediately after onboarding.  No network calls — purely informational.
 */

interface UploadGuideStepProps {
  /** Called when the user clicks "Got it". */
  onNext: () => void
}

const STEPS = [
  {
    number: '1',
    icon: '📄',
    title: 'Pick a subject',
    body: 'Open the Materials sidebar on the right side of the tutor screen.',
  },
  {
    number: '2',
    icon: '⬆️',
    title: 'Upload a file',
    body: 'Drag and drop a PDF, or click the upload button. Notes, slides, and textbook chapters all work.',
  },
  {
    number: '3',
    icon: '🤖',
    title: 'Ask questions',
    body: 'Your AI tutor reads the material and answers questions, explains concepts, and creates quizzes on it.',
  },
]

/**
 * UploadGuideStep — informational walkthrough for the materials upload flow.
 *
 * @param props.onNext - Advance to the next wizard step.
 */
export default function UploadGuideStep({ onNext }: UploadGuideStepProps) {
  return (
    <div className="flex flex-col gap-6">
      <div className="text-center">
        <h2 className="font-mono text-xl font-bold text-neon-pink mb-1">Upload your study materials</h2>
        <p className="text-sm text-text-dim">Your tutor learns from what you upload.</p>
      </div>

      <ul className="flex flex-col gap-4">
        {STEPS.map(({ number, icon, title, body }) => (
          <li key={number} className="flex gap-4 items-start">
            <div className="flex-shrink-0 w-8 h-8 rounded-full bg-neon-pink/10 border border-neon-pink text-neon-pink font-mono text-sm font-bold flex items-center justify-center">
              {number}
            </div>
            <div>
              <p className="text-sm font-semibold text-text-bright">
                {icon} {title}
              </p>
              <p className="text-xs text-text-dim mt-0.5">{body}</p>
            </div>
          </li>
        ))}
      </ul>

      <p className="text-[10px] text-text-dim text-center">
        Supported: PDF, TXT, Markdown — up to 50 MB per file
      </p>

      <button onClick={onNext} className="neon-btn-primary">
        Got it →
      </button>
    </div>
  )
}
