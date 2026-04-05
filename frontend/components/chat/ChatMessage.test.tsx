/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import ChatMessage, { type Message } from './ChatMessage'

function msg(overrides: Partial<Message> = {}): Message {
  return { id: '1', role: 'user', content: 'Hello', ...overrides }
}

describe('ChatMessage', () => {
  // ── Role styling ─────────────────────────────────────────────────────

  it('renders user avatar emoji for user messages', () => {
    render(<ChatMessage message={msg({ role: 'user' })} />)
    expect(screen.getByText('🧙')).toBeInTheDocument()
  })

  it('renders assistant avatar emoji for assistant messages', () => {
    render(<ChatMessage message={msg({ role: 'assistant', content: 'Hi' })} />)
    expect(screen.getByText('🤖')).toBeInTheDocument()
  })

  it('shows PROF NOVA label for assistant messages', () => {
    render(<ChatMessage message={msg({ role: 'assistant', content: 'Hi' })} />)
    expect(screen.getByText('PROF NOVA')).toBeInTheDocument()
  })

  it('does not show PROF NOVA label for user messages', () => {
    render(<ChatMessage message={msg({ role: 'user' })} />)
    expect(screen.queryByText('PROF NOVA')).not.toBeInTheDocument()
  })

  // ── Plain content ────────────────────────────────────────────────────

  it('renders plain text content', () => {
    render(<ChatMessage message={msg({ content: 'What is photosynthesis?' })} />)
    expect(screen.getByText('What is photosynthesis?')).toBeInTheDocument()
  })

  // ── Markdown: bold ───────────────────────────────────────────────────

  it('renders **bold** text as <strong>', () => {
    render(<ChatMessage message={msg({ content: '**important**' })} />)
    const el = screen.getByText('important')
    expect(el.tagName).toBe('STRONG')
  })

  // ── Markdown: italic ─────────────────────────────────────────────────

  it('renders *italic* text as <em>', () => {
    render(<ChatMessage message={msg({ content: '*slanted*' })} />)
    const el = screen.getByText('slanted')
    expect(el.tagName).toBe('EM')
  })

  // ── Markdown: inline code ─────────────────────────────────────────────

  it('renders `code` as <code>', () => {
    render(<ChatMessage message={msg({ content: '`const x = 1`' })} />)
    const el = screen.getByText('const x = 1')
    expect(el.tagName).toBe('CODE')
  })

  // ── Streaming cursor class ────────────────────────────────────────────

  it('applies typing-cursor class when streaming is true', () => {
    const { container } = render(
      <ChatMessage message={msg({ role: 'assistant', content: 'typing…', streaming: true })} />
    )
    expect(container.querySelector('.typing-cursor')).not.toBeNull()
  })

  it('does not apply typing-cursor class when streaming is false', () => {
    const { container } = render(
      <ChatMessage message={msg({ role: 'assistant', content: 'done', streaming: false })} />
    )
    expect(container.querySelector('.typing-cursor')).toBeNull()
  })
})
