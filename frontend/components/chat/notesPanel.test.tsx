/**
 * @jest-environment jsdom
 *
 * Unit tests for NotesPanel — the read/write-mode structured notes component.
 */
import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import NotesPanel, { type NoteEntry, exportAsMarkdown, exportAsAnkiCsv } from './NotesPanel'

// ── Fixtures ──────────────────────────────────────────────────────────────────

const DEFINITION: NoteEntry = {
  type: 'definition',
  term: 'Photosynthesis',
  body: 'The process by which plants convert sunlight, CO₂, and H₂O into glucose and O₂.',
}

const RULE: NoteEntry = {
  type: 'rule',
  term: 'Law of Conservation of Mass',
  body: 'Matter is neither created nor destroyed in a chemical reaction.',
}

const EXAMPLE: NoteEntry = {
  type: 'example',
  term: 'Photosynthesis equation',
  body: '6CO₂ + 6H₂O + light → C₆H₁₂O₆ + 6O₂',
}

const FLASHCARD: NoteEntry = {
  type: 'flashcard',
  term: 'What is ATP?',
  body: 'Adenosine triphosphate — the primary energy currency of the cell.',
}

const ALL_ENTRIES: NoteEntry[] = [DEFINITION, RULE, EXAMPLE, FLASHCARD]

// ── Rendering ─────────────────────────────────────────────────────────────────

describe('NotesPanel — rendering', () => {
  it('renders without crashing', () => {
    render(<NotesPanel notes={[DEFINITION]} />)
  })

  it('renders the panel heading', () => {
    render(<NotesPanel notes={[DEFINITION]} />)
    expect(screen.getByRole('heading', { name: /notes/i })).toBeInTheDocument()
  })

  it('renders term for each note entry', () => {
    render(<NotesPanel notes={ALL_ENTRIES} />)
    expect(screen.getByText('Photosynthesis')).toBeInTheDocument()
    expect(screen.getByText('Law of Conservation of Mass')).toBeInTheDocument()
    expect(screen.getByText('Photosynthesis equation')).toBeInTheDocument()
    expect(screen.getByText('What is ATP?')).toBeInTheDocument()
  })

  it('renders body for each note entry', () => {
    render(<NotesPanel notes={[DEFINITION]} />)
    expect(screen.getByText(/The process by which plants convert sunlight/)).toBeInTheDocument()
  })

  it('renders a type label for definitions', () => {
    render(<NotesPanel notes={[DEFINITION]} />)
    expect(screen.getByText('DEF')).toBeInTheDocument()
  })

  it('renders a type label for rules', () => {
    render(<NotesPanel notes={[RULE]} />)
    expect(screen.getByText('RULE')).toBeInTheDocument()
  })

  it('renders a type label for examples', () => {
    render(<NotesPanel notes={[EXAMPLE]} />)
    expect(screen.getByText('EX')).toBeInTheDocument()
  })

  it('renders a type label for flashcards', () => {
    render(<NotesPanel notes={[FLASHCARD]} />)
    expect(screen.getByText('CARD')).toBeInTheDocument()
  })

  it('renders empty state message when notes array is empty', () => {
    render(<NotesPanel notes={[]} />)
    expect(screen.getByText(/no notes yet/i)).toBeInTheDocument()
  })
})

// ── Flashcard flip ────────────────────────────────────────────────────────────

describe('NotesPanel — flashcard flip', () => {
  it('shows the term (front) of a flashcard by default', () => {
    render(<NotesPanel notes={[FLASHCARD]} />)
    expect(screen.getByText('What is ATP?')).toBeInTheDocument()
  })

  it('shows the answer (body) after clicking a flashcard', () => {
    render(<NotesPanel notes={[FLASHCARD]} />)
    const card = screen.getByRole('button', { name: /flip card/i })
    fireEvent.click(card)
    expect(
      screen.getByText(/Adenosine triphosphate — the primary energy currency/),
    ).toBeInTheDocument()
  })

  it('flips back to the term when clicked a second time', () => {
    render(<NotesPanel notes={[FLASHCARD]} />)
    const card = screen.getByRole('button', { name: /flip card/i })
    fireEvent.click(card)
    fireEvent.click(card)
    expect(screen.getByText('What is ATP?')).toBeInTheDocument()
  })
})

// ── Export buttons ────────────────────────────────────────────────────────────

describe('NotesPanel — export buttons', () => {
  it('calls onExport with "markdown" when Markdown button is clicked', () => {
    const onExport = jest.fn()
    render(<NotesPanel notes={ALL_ENTRIES} onExport={onExport} />)
    fireEvent.click(screen.getByRole('button', { name: /markdown/i }))
    expect(onExport).toHaveBeenCalledWith('markdown', expect.any(String))
  })

  it('calls onExport with "anki" when Anki button is clicked', () => {
    const onExport = jest.fn()
    render(<NotesPanel notes={ALL_ENTRIES} onExport={onExport} />)
    fireEvent.click(screen.getByRole('button', { name: /anki/i }))
    expect(onExport).toHaveBeenCalledWith('anki', expect.any(String))
  })

  it('does not render export buttons when onExport prop is absent', () => {
    render(<NotesPanel notes={ALL_ENTRIES} />)
    expect(screen.queryByRole('button', { name: /markdown/i })).toBeNull()
    expect(screen.queryByRole('button', { name: /anki/i })).toBeNull()
  })
})

// ── Export utilities ──────────────────────────────────────────────────────────

describe('exportAsMarkdown', () => {
  it('includes a heading', () => {
    const md = exportAsMarkdown([DEFINITION])
    expect(md).toMatch(/^#/)
  })

  it('includes each term and body', () => {
    const md = exportAsMarkdown([DEFINITION, RULE])
    expect(md).toContain('Photosynthesis')
    expect(md).toContain('Law of Conservation of Mass')
    expect(md).toContain('The process by which plants convert sunlight')
  })

  it('returns empty string for empty notes array', () => {
    expect(exportAsMarkdown([])).toBe('')
  })
})

describe('exportAsAnkiCsv', () => {
  it('includes a header row', () => {
    const csv = exportAsAnkiCsv([FLASHCARD])
    expect(csv.split('\n')[0]).toMatch(/front.*back/i)
  })

  it('includes one data row per flashcard entry', () => {
    const csv = exportAsAnkiCsv([FLASHCARD, DEFINITION])
    // Only flashcard-type entries go to Anki; definition counts too (front=term, back=body)
    const rows = csv.trim().split('\n')
    expect(rows.length).toBeGreaterThanOrEqual(2) // header + at least 1
  })

  it('includes term in front column', () => {
    const csv = exportAsAnkiCsv([FLASHCARD])
    expect(csv).toContain('What is ATP?')
  })

  it('includes body in back column', () => {
    const csv = exportAsAnkiCsv([FLASHCARD])
    expect(csv).toContain('Adenosine triphosphate')
  })

  it('returns only header for empty notes array', () => {
    const csv = exportAsAnkiCsv([])
    expect(csv.trim().split('\n').length).toBe(1)
  })
})
