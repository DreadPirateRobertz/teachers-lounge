/**
 * @jest-environment jsdom
 *
 * Unit tests for ResponseRenderer — diagram-tag parsing and rendering for
 * the multi-modal tutor response layer (tl-h9e).
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import ResponseRenderer, { parseDiagramTags, DiagramPlaceholder } from './ResponseRenderer'

describe('parseDiagramTags', () => {
  it('returns a single text segment when no tags present', () => {
    const segs = parseDiagramTags('Hello world!')
    expect(segs).toEqual([{ type: 'text', value: 'Hello world!' }])
  })

  it('returns an empty text segment for empty input', () => {
    const segs = parseDiagramTags('')
    expect(segs).toEqual([{ type: 'text', value: '' }])
  })

  it('extracts a single diagram tag with surrounding text', () => {
    const segs = parseDiagramTags('Look here [DIAGRAM: photosynthesis cycle] then continue.')
    expect(segs).toEqual([
      { type: 'text', value: 'Look here ' },
      { type: 'diagram', description: 'photosynthesis cycle' },
      { type: 'text', value: ' then continue.' },
    ])
  })

  it('extracts multiple diagram tags in order', () => {
    const segs = parseDiagramTags('A [DIAGRAM: one] B [DIAGRAM: two] C')
    expect(segs).toEqual([
      { type: 'text', value: 'A ' },
      { type: 'diagram', description: 'one' },
      { type: 'text', value: ' B ' },
      { type: 'diagram', description: 'two' },
      { type: 'text', value: ' C' },
    ])
  })

  it('trims internal whitespace in descriptions', () => {
    const segs = parseDiagramTags('[DIAGRAM:   spaced description   ]')
    expect(segs).toEqual([{ type: 'diagram', description: 'spaced description' }])
  })

  it('handles a tag at the end of the string', () => {
    const segs = parseDiagramTags('Final note: [DIAGRAM: end]')
    expect(segs).toEqual([
      { type: 'text', value: 'Final note: ' },
      { type: 'diagram', description: 'end' },
    ])
  })

  it('does not match malformed tags missing a colon', () => {
    const segs = parseDiagramTags('No match: [DIAGRAM no colon]')
    expect(segs).toEqual([{ type: 'text', value: 'No match: [DIAGRAM no colon]' }])
  })

  it('is reentrant — repeated calls return identical results', () => {
    const text = 'A [DIAGRAM: x] B'
    expect(parseDiagramTags(text)).toEqual(parseDiagramTags(text))
  })
})

describe('DiagramPlaceholder', () => {
  it('renders an accessible figure with the description as caption', () => {
    render(<DiagramPlaceholder description="cell membrane diffusion" />)
    const fig = screen.getByTestId('diagram-placeholder')
    expect(fig).toHaveAttribute('aria-label', 'Diagram: cell membrane diffusion')
    expect(screen.getByText('cell membrane diffusion')).toBeInTheDocument()
  })
})

describe('ResponseRenderer', () => {
  it('renders plain text via the default renderer when no tags present', () => {
    render(<ResponseRenderer text="Just plain text." />)
    expect(screen.getByText('Just plain text.')).toBeInTheDocument()
    expect(screen.queryByTestId('diagram-placeholder')).not.toBeInTheDocument()
  })

  it('renders diagram placeholder for [DIAGRAM:] tag', () => {
    render(<ResponseRenderer text="See [DIAGRAM: alpha helix] above." />)
    expect(screen.getByTestId('diagram-placeholder')).toBeInTheDocument()
    expect(screen.getByText('alpha helix')).toBeInTheDocument()
  })

  it('forwards text segments to a custom renderTextSegment', () => {
    render(
      <ResponseRenderer
        text="Hello [DIAGRAM: x] World"
        renderTextSegment={(s, key) => (
          <span key={key} data-testid={`custom-${key}`}>
            CUSTOM:{s}
          </span>
        )}
      />,
    )
    expect(screen.getByTestId('custom-t-0')).toHaveTextContent('CUSTOM:Hello')
    expect(screen.getByTestId('custom-t-2')).toHaveTextContent('CUSTOM: World')
    expect(screen.getByTestId('diagram-placeholder')).toBeInTheDocument()
  })

  it('renders multiple diagram placeholders in order', () => {
    render(<ResponseRenderer text="[DIAGRAM: first][DIAGRAM: second]" />)
    const placeholders = screen.getAllByTestId('diagram-placeholder')
    expect(placeholders).toHaveLength(2)
    expect(placeholders[0]).toHaveAttribute('aria-label', 'Diagram: first')
    expect(placeholders[1]).toHaveAttribute('aria-label', 'Diagram: second')
  })
})
