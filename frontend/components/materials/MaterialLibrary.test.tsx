/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import MaterialLibrary from './MaterialLibrary'
import type { UploadedMaterial } from './MaterialUpload'

function makeMaterial(overrides: Partial<UploadedMaterial> = {}): UploadedMaterial {
  return {
    jobId: 'job-1',
    filename: 'lecture-notes.pdf',
    fileType: 'PDF',
    status: 'complete',
    uploadedAt: new Date().toISOString(),
    ...overrides,
  }
}

describe('MaterialLibrary — empty state', () => {
  it('shows empty state message when no materials', () => {
    render(<MaterialLibrary materials={[]} />)
    expect(screen.getByText('No materials yet')).toBeInTheDocument()
  })

  it('shows upload prompt in empty state', () => {
    render(<MaterialLibrary materials={[]} />)
    expect(screen.getByText(/Upload a file above/)).toBeInTheDocument()
  })
})

describe('MaterialLibrary — with materials', () => {
  it('renders filename', () => {
    render(<MaterialLibrary materials={[makeMaterial()]} />)
    expect(screen.getByText('lecture-notes.pdf')).toBeInTheDocument()
  })

  it('renders file type', () => {
    render(<MaterialLibrary materials={[makeMaterial({ fileType: 'DOCX' })]} />)
    expect(screen.getByText(/DOCX/)).toBeInTheDocument()
  })

  it('renders "Ready" label for complete status', () => {
    render(<MaterialLibrary materials={[makeMaterial({ status: 'complete' })]} />)
    expect(screen.getByText('Ready')).toBeInTheDocument()
  })

  it('renders "Pending" label for pending status', () => {
    render(<MaterialLibrary materials={[makeMaterial({ status: 'pending' })]} />)
    expect(screen.getByText('Pending')).toBeInTheDocument()
  })

  it('renders "Processing" label for processing status', () => {
    render(<MaterialLibrary materials={[makeMaterial({ status: 'processing' })]} />)
    expect(screen.getByText('Processing')).toBeInTheDocument()
  })

  it('renders "Failed" label for failed status', () => {
    render(<MaterialLibrary materials={[makeMaterial({ status: 'failed' })]} />)
    expect(screen.getByText('Failed')).toBeInTheDocument()
  })

  it('renders multiple materials', () => {
    render(<MaterialLibrary materials={[
      makeMaterial({ jobId: 'j1', filename: 'file-a.pdf' }),
      makeMaterial({ jobId: 'j2', filename: 'file-b.docx', fileType: 'DOCX' }),
    ]} />)
    expect(screen.getByText('file-a.pdf')).toBeInTheDocument()
    expect(screen.getByText('file-b.docx')).toBeInTheDocument()
  })

  it('shows PDF emoji for PDF files', () => {
    render(<MaterialLibrary materials={[makeMaterial({ fileType: 'PDF' })]} />)
    expect(screen.getByText('📄')).toBeInTheDocument()
  })

  it('shows video emoji for MP4 files', () => {
    render(<MaterialLibrary materials={[makeMaterial({ fileType: 'MP4' })]} />)
    expect(screen.getByText('🎬')).toBeInTheDocument()
  })

  it('shows "just now" for a material uploaded right now', () => {
    render(<MaterialLibrary materials={[makeMaterial({ uploadedAt: new Date().toISOString() })]} />)
    expect(screen.getByText(/just now/)).toBeInTheDocument()
  })
})
