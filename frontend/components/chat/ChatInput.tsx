'use client'

import React, { useRef } from 'react'

interface Props {
  value: string
  onChange: (v: string) => void
  onSubmit: () => void
  disabled?: boolean
}

export default function ChatInput({ value, onChange, onSubmit, disabled }: Props) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (!disabled && value.trim()) onSubmit()
    }
  }

  return (
    <form
      onSubmit={(e) => { e.preventDefault(); if (!disabled && value.trim()) onSubmit() }}
      className="flex gap-2 p-3 border-t border-border-dim bg-bg-panel"
    >
      <div className="flex-1 relative">
        <textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={disabled}
          placeholder="Ask Prof Nova anything... ⚡"
          rows={1}
          className="w-full bg-bg-input border border-border-mid rounded-lg px-3.5 py-2.5 text-sm text-text-base placeholder:text-text-dim resize-none focus:outline-none focus:border-neon-blue/60 focus:shadow-neon-blue-sm transition-all leading-relaxed disabled:opacity-50"
          style={{ minHeight: '42px', maxHeight: '120px' }}
        />
      </div>
      <button
        type="submit"
        disabled={disabled || !value.trim()}
        className="flex-shrink-0 w-10 h-10 rounded-lg bg-neon-blue/10 border border-neon-blue/40 text-neon-blue hover:bg-neon-blue/20 hover:shadow-neon-blue-sm disabled:opacity-30 disabled:cursor-not-allowed transition-all flex items-center justify-center self-end mb-0.5"
        title="Send (Enter)"
      >
        <SendIcon />
      </button>
    </form>
  )
}

function SendIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <line x1="22" y1="2" x2="11" y2="13" />
      <polygon points="22 2 15 22 11 13 2 9 22 2" />
    </svg>
  )
}
