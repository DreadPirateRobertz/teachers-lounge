'use client'

import { type ReactNode } from 'react'

/**
 * Matches a diagram embed tag: `[DIAGRAM: description]`.
 * The description is captured (group 1) and may contain spaces,
 * but no newlines or `]` characters.
 */
const DIAGRAM_TAG_RE = /\[DIAGRAM:\s*([^\]\n]+?)\s*\]/g

/** A parsed segment of a tutor response. */
export type Segment = { type: 'text'; value: string } | { type: 'diagram'; description: string }

/**
 * Split a tutor response into ordered text and diagram segments.
 *
 * Each `[DIAGRAM: description]` tag becomes a `diagram` segment; everything
 * else is preserved verbatim as `text` segments. Empty text segments between
 * adjacent tags are dropped.
 *
 * @param text - Raw tutor response, possibly containing diagram tags.
 * @returns Ordered array of text and diagram segments.
 */
export function parseDiagramTags(text: string): Segment[] {
  DIAGRAM_TAG_RE.lastIndex = 0
  const segments: Segment[] = []
  let last = 0
  let match: RegExpExecArray | null
  while ((match = DIAGRAM_TAG_RE.exec(text)) !== null) {
    if (match.index > last) {
      segments.push({ type: 'text', value: text.slice(last, match.index) })
    }
    segments.push({ type: 'diagram', description: match[1].trim() })
    last = match.index + match[0].length
  }
  if (last < text.length) {
    segments.push({ type: 'text', value: text.slice(last) })
  }
  if (segments.length === 0) {
    segments.push({ type: 'text', value: '' })
  }
  return segments
}

/**
 * Inline placeholder shown for `[DIAGRAM: description]` tags.
 *
 * Renders a labeled neon panel with the description as caption and a
 * stylized "diagram" hint. The placeholder is keyboard-accessible and
 * announced as a figure for screen readers.
 */
export function DiagramPlaceholder({ description }: { description: string }) {
  return (
    <figure
      role="img"
      aria-label={`Diagram: ${description}`}
      data-testid="diagram-placeholder"
      className="my-3 p-3 rounded-lg border border-neon-pink/40 bg-neon-pink/5 flex flex-col items-center gap-2"
    >
      <div className="text-2xl select-none" aria-hidden>
        🖼️
      </div>
      <figcaption className="text-[10px] font-mono text-neon-pink text-center italic">
        {description}
      </figcaption>
    </figure>
  )
}

interface Props {
  /** Raw tutor response that may contain `[DIAGRAM: ...]` tags. */
  text: string
  /**
   * Renderer used for plain-text segments. Lets the caller plug in markdown,
   * math, or molecule rendering. Defaults to a plain `<span>`.
   */
  renderTextSegment?: (segment: string, key: string) => ReactNode
}

/**
 * Wraps a tutor message body and renders inline diagram placeholders for any
 * `[DIAGRAM: description]` tags it contains. Non-diagram text is forwarded to
 * the caller-supplied `renderTextSegment` (or rendered as plain text by
 * default), so this component composes with the existing markdown / math /
 * molecule rendering pipeline in `ChatMessage`.
 */
export default function ResponseRenderer({ text, renderTextSegment }: Props) {
  const segments = parseDiagramTags(text)
  const renderText = renderTextSegment ?? ((s: string, key: string) => <span key={key}>{s}</span>)
  return (
    <>
      {segments.map((seg, i) =>
        seg.type === 'diagram' ? (
          <DiagramPlaceholder key={`d-${i}`} description={seg.description} />
        ) : (
          renderText(seg.value, `t-${i}`)
        ),
      )}
    </>
  )
}
