'use client'

import { useState } from 'react'

/**
 * Wizard step for character creation.
 *
 * Lets the user pick an avatar emoji and optionally update their display name.
 * The selections are saved to the user service via PATCH /preferences on submit.
 */

/** Emoji options grouped by theme for the avatar picker. */
const AVATAR_OPTIONS = [
  '🧙',
  '🦊',
  '🐉',
  '⚗️',
  '🔭',
  '🧬',
  '🤖',
  '👾',
  '🦁',
  '🐺',
  '🦅',
  '🌟',
  '⚡',
  '🔮',
  '🎯',
  '🛸',
]

interface CharacterStepProps {
  /** Current display name (from registration). */
  displayName: string
  /** Current avatar emoji (default from user service). */
  avatarEmoji: string
  /** Called with updated values when the user saves. */
  onNext: (displayName: string, avatarEmoji: string) => Promise<void>
}

/**
 * CharacterStep — avatar picker and display name editor.
 *
 * @param props.displayName - Initial display name.
 * @param props.avatarEmoji - Initial avatar emoji.
 * @param props.onNext - Async callback called with the chosen values.
 */
export default function CharacterStep({ displayName, avatarEmoji, onNext }: CharacterStepProps) {
  const [name, setName] = useState(displayName)
  const [avatar, setAvatar] = useState(avatarEmoji || '🎓')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) {
      setError('Display name is required')
      return
    }
    setSaving(true)
    setError('')
    try {
      await onNext(name.trim(), avatar)
    } catch {
      setError('Failed to save. Please try again.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-6">
      <div className="text-center">
        <h2 className="font-mono text-xl font-bold text-neon-blue mb-1">Create your character</h2>
        <p className="text-sm text-text-dim">Choose your avatar and confirm your name.</p>
      </div>

      {/* Avatar preview */}
      <div className="flex flex-col items-center gap-2">
        <div className="w-20 h-20 rounded-full bg-bg-card border-2 border-neon-blue shadow-neon-blue flex items-center justify-center text-5xl">
          {avatar}
        </div>
        <span className="text-xs text-text-dim">Your avatar</span>
      </div>

      {/* Avatar picker grid */}
      <div className="grid grid-cols-8 gap-2">
        {AVATAR_OPTIONS.map((emoji) => (
          <button
            key={emoji}
            type="button"
            aria-label={`Select avatar ${emoji}`}
            aria-pressed={avatar === emoji}
            onClick={() => setAvatar(emoji)}
            className={`text-2xl p-1.5 rounded-lg transition-colors ${
              avatar === emoji
                ? 'bg-neon-blue/20 border border-neon-blue shadow-neon-blue'
                : 'bg-bg-card border border-border-mid hover:border-neon-blue/50'
            }`}
          >
            {emoji}
          </button>
        ))}
      </div>

      {/* Display name */}
      <div>
        <label htmlFor="onboard-name" className="block text-xs font-medium text-text-dim mb-1.5">
          Display Name
        </label>
        <input
          id="onboard-name"
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={40}
          className="neon-input w-full"
        />
      </div>

      {error && (
        <div className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded px-3 py-2">
          {error}
        </div>
      )}

      <button type="submit" disabled={saving} className="neon-btn-primary">
        {saving ? 'Saving…' : 'Looks good →'}
      </button>
    </form>
  )
}
