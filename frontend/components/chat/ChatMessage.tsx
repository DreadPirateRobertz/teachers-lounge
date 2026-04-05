'use client'

import { type ReactNode, useState } from 'react'
import MathBlock from './MathBlock'

export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  streaming?: boolean
  /** Diagrams returned by the CLIP diagram search (Phase 6). */
  diagrams?: DiagramAttachment[]
}

export interface DiagramAttachment {
  diagram_id: string
  gcs_path: string
  caption: string
  figure_type: string
  score: number
}

interface Props {
  message: Message
}

// ── Content rendering helpers ─────────────────────────────────────────────────

/**
 * Split text at LaTeX display blocks ($$...$$) and render each segment.
 * Segments between delimiters are rendered as MathBlock (block mode).
 */
function renderWithBlockMath(text: string): ReactNode[] {
  const parts = text.split(/(\\$\\$[\s\S]*?\\$\\$)/g)
  return parts.map((part, i) => {
    if (part.startsWith('$$') && part.endsWith('$$')) {
      return <MathBlock key={i} expression={part.slice(2, -2)} block />
    }
    return <span key={i}>{renderWithInlineMath(part)}</span>
  })
}

/**
 * Split text at inline LaTeX ($...$) and render each segment.
 * Protects against $$...$$ by requiring the $ not be adjacent to another $.
 */
function renderWithInlineMath(text: string): ReactNode[] {
  // Match $...$ not preceded or followed by another $
  const parts = text.split(/(?<!\$)\$([^$\n]+?)\$(?!\$)/g)
  const nodes: ReactNode[] = []
  for (let i = 0; i < parts.length; i++) {
    if (i % 2 === 0) {
      // Plain text segment — apply markdown formatting
      nodes.push(...renderInline(parts[i], i * 100))
    } else {
      // Odd segments are the captured group (inside $...$)
      nodes.push(<MathBlock key={i * 100 + 50} expression={parts[i]} />)
    }
  }
  return nodes
}

/** Render a single line of content, applying bold/italic/code markdown. */
function renderInline(text: string, keyOffset: number): ReactNode[] {
  const parts = text.split(/(\*\*[^*]+\*\*|\*[^*]+\*|`[^`]+`)/g)
  return parts.map((part, i) => {
    const key = keyOffset + i
    if (part.startsWith('**') && part.endsWith('**')) {
      return (
        <strong key={key} className="font-bold text-text-bright">
          {part.slice(2, -2)}
        </strong>
      )
    }
    if (part.startsWith('*') && part.endsWith('*')) {
      return (
        <em key={key} className="italic text-text-base">
          {part.slice(1, -1)}
        </em>
      )
    }
    if (part.startsWith('`') && part.endsWith('`')) {
      return (
        <code
          key={key}
          className="font-mono text-xs bg-bg-deep border border-border-dim px-1 py-0.5 rounded text-neon-green"
        >
          {part.slice(1, -1)}
        </code>
      )
    }
    return <span key={key}>{part}</span>
  })
}

/**
 * Render message content: split on newlines, apply block/inline math and
 * markdown to each line.
 */
function renderContent(text: string): ReactNode {
  const lines = text.split('\n')
  return lines.map((line, i) => (
    <span key={i}>
      {renderWithBlockMath(line)}
      {i < lines.length - 1 && <br />}
    </span>
  ))
}

// ── Diagram component ─────────────────────────────────────────────────────────

/**
 * Renders a single retrieved diagram inline in the chat message.
 * Clicking the thumbnail opens it at full size (zoom-on-click).
 */
function DiagramCard({ diagram }: { diagram: DiagramAttachment }) {
  const [zoomed, setZoomed] = useState(false)

  // The gcs_path is a gs:// URI; in production the backend signs this into an
  // HTTPS URL.  In dev/test we use it as-is and let the browser show a broken
  // image (harmless for testing layout).
  const src = diagram.gcs_path.startsWith('gs://')
    ? diagram.gcs_path.replace('gs://', 'https://storage.googleapis.com/')
    : diagram.gcs_path

  return (
    <>
      <figure
        className="mt-3 cursor-zoom-in max-w-sm"
        onClick={() => setZoomed(true)}
        role="button"
        aria-label={`Zoom diagram: ${diagram.caption}`}
      >
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={src}
          alt={diagram.caption}
          className="rounded-lg border border-border-dim max-h-48 object-contain bg-bg-deep"
        />
        {diagram.caption && (
          <figcaption className="mt-1 text-[10px] text-text-dim italic">
            {diagram.caption}
          </figcaption>
        )}
      </figure>

      {/* Full-size overlay */}
      {zoomed && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 cursor-zoom-out"
          onClick={() => setZoomed(false)}
          role="dialog"
          aria-label="Diagram full view"
        >
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={src}
            alt={diagram.caption}
            className="max-w-[90vw] max-h-[90vh] rounded-xl border border-border-mid object-contain"
          />
        </div>
      )}
    </>
  )
}

// ── ChatMessage ───────────────────────────────────────────────────────────────

/**
 * Renders a single chat message bubble.
 *
 * Supports:
 * - User and assistant roles with matching avatars and styles.
 * - Markdown: **bold**, *italic*, `code`.
 * - LaTeX: inline $...$ and block $$...$$ via KaTeX (MathBlock component).
 * - Inline diagrams with zoom-on-click (Phase 6 CLIP retrieval).
 * - Streaming typing cursor animation.
 */
export default function ChatMessage({ message }: Props) {
  const isUser = message.role === 'user'

  return (
    <div className={`flex gap-3 animate-slide-up ${isUser ? 'flex-row-reverse' : 'flex-row'}`}>
      {/* Avatar */}
      <div
        className={`flex-shrink-0 w-7 h-7 rounded-full flex items-center justify-center text-sm border ${
          isUser
            ? 'bg-bg-card border-border-mid'
            : 'bg-neon-blue/10 border-neon-blue/30 shadow-neon-blue-sm'
        }`}
      >
        {isUser ? '🧙' : '🤖'}
      </div>

      {/* Bubble */}
      <div
        className={`max-w-[75%] rounded-xl px-3.5 py-2.5 text-sm leading-relaxed ${
          isUser
            ? 'bg-bg-card border border-border-mid text-text-base rounded-tr-sm'
            : 'bg-neon-blue/5 border border-neon-blue/20 text-text-base rounded-tl-sm'
        }`}
      >
        {!isUser && (
          <div className="text-[10px] font-mono text-neon-blue mb-1.5 font-bold">PROF NOVA</div>
        )}
        <div className={message.streaming ? 'typing-cursor' : ''}>
          {renderContent(message.content)}
        </div>

        {/* Inline diagrams (Phase 6) */}
        {!isUser && message.diagrams && message.diagrams.length > 0 && (
          <div className="mt-2 flex flex-col gap-2">
            {message.diagrams.map((d) => (
              <DiagramCard key={d.diagram_id} diagram={d} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
