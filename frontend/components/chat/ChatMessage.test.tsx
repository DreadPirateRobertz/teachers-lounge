/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import ChatMessage, { type Message } from './ChatMessage'

// MoleculeViewer is loaded via next/dynamic (ssr:false); render a stub so
// tests run without WebGL.
jest.mock(
  'next/dynamic',
  () => (_loader: unknown, _opts?: unknown) =>
    function DynamicStub() {
      return null
    },
)

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
      <ChatMessage message={msg({ role: 'assistant', content: 'typing…', streaming: true })} />,
    )
    expect(container.querySelector('.typing-cursor')).not.toBeNull()
  })

  it('does not apply typing-cursor class when streaming is false', () => {
    const { container } = render(
      <ChatMessage message={msg({ role: 'assistant', content: 'done', streaming: false })} />,
    )
    expect(container.querySelector('.typing-cursor')).toBeNull()
  })

  // ── Molecule viewer integration ────────────────────────────────────────

  it('strips [molecule:...] tag — raw tag text not visible in DOM', () => {
    render(<ChatMessage message={msg({ content: 'Look at this [molecule:water]' })} />)
    expect(screen.queryByText(/\[molecule:water\]/)).toBeNull()
  })

  it('renders text around [molecule:...] tag unchanged', () => {
    render(<ChatMessage message={msg({ content: 'Before [molecule:benzene] after' })} />)
    // Text is rendered inside nested spans; use regex to match across
    // element boundaries after whitespace normalization.
    expect(screen.getByText(/Before/)).toBeInTheDocument()
    expect(screen.getByText(/after/)).toBeInTheDocument()
  })

  it('handles multiple molecule tags in one message', () => {
    render(<ChatMessage message={msg({ content: '[molecule:water] and [molecule:co2]' })} />)
    expect(screen.queryByText(/\[molecule:/)).toBeNull()
  })

  it('renders plain text unchanged when no molecule tags present', () => {
    render(<ChatMessage message={msg({ content: 'No special tags here.' })} />)
    expect(screen.getByText('No special tags here.')).toBeInTheDocument()
  })

  // ── Diagram attachments (Phase 6 CLIP) ────────────────────────────────

  it('renders diagram caption for assistant messages with diagrams', () => {
    render(
      <ChatMessage
        message={msg({
          role: 'assistant',
          content: 'Here is a related diagram:',
          diagrams: [
            {
              diagram_id: 'abc',
              gcs_path: 'gs://bucket/fig.png',
              caption: 'Benzene ring',
              figure_type: 'diagram',
              score: 0.9,
            },
          ],
        })}
      />,
    )
    expect(screen.getByText('Benzene ring')).toBeInTheDocument()
  })

  it('does not render diagram section for user messages even if diagrams passed', () => {
    render(
      <ChatMessage
        message={msg({
          role: 'user',
          content: 'Hi',
          diagrams: [
            {
              diagram_id: 'abc',
              gcs_path: 'gs://bucket/fig.png',
              caption: 'Should not appear',
              figure_type: 'diagram',
              score: 0.9,
            },
          ],
        })}
      />,
    )
    expect(screen.queryByText('Should not appear')).toBeNull()
  })
})

// ── TTS audio player integration ──────────────────────────────────────────────

describe('ChatMessage — TTS audio player', () => {
  beforeAll(() => {
    Object.defineProperty(HTMLMediaElement.prototype, 'play', {
      configurable: true,
      value: jest.fn().mockResolvedValue(undefined),
    })
    Object.defineProperty(HTMLMediaElement.prototype, 'pause', {
      configurable: true,
      value: jest.fn(),
    })
    Object.defineProperty(HTMLMediaElement.prototype, 'load', {
      configurable: true,
      value: jest.fn(),
    })
  })

  it('renders TTS player for assistant messages with audioUrl', () => {
    render(
      <ChatMessage
        message={msg({
          role: 'assistant',
          content: 'Here is the explanation.',
          audioUrl: 'https://tts.example.com/audio/abc.mp3',
        })}
      />,
    )
    expect(screen.getByRole('button', { name: /play/i })).toBeInTheDocument()
  })

  it('does not render TTS player for user messages', () => {
    render(
      <ChatMessage
        message={msg({
          role: 'user',
          content: 'Hello',
          audioUrl: 'https://tts.example.com/audio/abc.mp3',
        })}
      />,
    )
    expect(screen.queryByRole('button', { name: /play/i })).toBeNull()
  })

  it('does not render TTS player when audioUrl is absent', () => {
    render(<ChatMessage message={msg({ role: 'assistant', content: 'No audio here.' })} />)
    expect(screen.queryByRole('button', { name: /play/i })).toBeNull()
  })

  it('renders phrase bookmarks when audioPhrases are provided', () => {
    render(
      <ChatMessage
        message={msg({
          role: 'assistant',
          content: 'Listen carefully.',
          audioUrl: 'https://tts.example.com/audio/abc.mp3',
          audioPhrases: [{ text: 'Key concept here.', startMs: 3000 }],
        })}
      />,
    )
    expect(screen.getByText('Key concept here.')).toBeInTheDocument()
  })
})

// ── NotesPanel integration ─────────────────────────────────────────────────────

describe('ChatMessage — NotesPanel', () => {
  it('renders notes panel for assistant messages with notes', () => {
    render(
      <ChatMessage
        message={msg({
          role: 'assistant',
          content: 'See the notes below.',
          notes: [
            { type: 'definition', term: 'Osmosis', body: 'Diffusion of water through a membrane.' },
          ],
        })}
      />,
    )
    expect(screen.getByRole('heading', { name: /notes/i })).toBeInTheDocument()
    expect(screen.getByText('Osmosis')).toBeInTheDocument()
  })

  it('does not render notes panel for user messages', () => {
    render(
      <ChatMessage
        message={msg({
          role: 'user',
          content: 'Hi',
          notes: [{ type: 'definition', term: 'Should not appear', body: '...' }],
        })}
      />,
    )
    expect(screen.queryByText('Should not appear')).toBeNull()
  })

  it('does not render notes panel when notes array is empty', () => {
    render(<ChatMessage message={msg({ role: 'assistant', content: 'No notes.', notes: [] })} />)
    expect(screen.queryByRole('heading', { name: /notes/i })).toBeNull()
  })
})
