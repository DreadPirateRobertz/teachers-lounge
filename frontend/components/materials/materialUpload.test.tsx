/**
 * @fileoverview Tests for the MaterialUpload component.
 *
 * Covers drop zone rendering, file type validation, upload flow,
 * success/error handling, and file removal.
 *
 * @jest-environment jsdom
 */

import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import MaterialUpload, { UploadedMaterial } from './MaterialUpload'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Creates a fake File object with the given name and MIME type.
 *
 * @param name - Filename (e.g. 'lecture.pdf').
 * @param type - MIME type (e.g. 'application/pdf').
 * @returns A File instance suitable for use in tests.
 */
function makeFile(name: string, type: string): File {
  return new File(['content'], name, { type })
}

/**
 * Builds a minimal DataTransfer stub for simulating drag-and-drop events.
 *
 * @param file - The file to include in the transfer.
 * @returns An object shaped like a DataTransfer with a files list.
 */
function makeDataTransfer(file: File): Partial<DataTransfer> {
  return {
    files: [file] as unknown as FileList,
    dropEffect: 'copy',
    effectAllowed: 'all',
  }
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

const mockFetch = jest.fn()

beforeEach(() => {
  global.fetch = mockFetch
})

afterEach(() => {
  mockFetch.mockReset()
  jest.restoreAllMocks()
})

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('MaterialUpload', () => {
  const courseId = 'course-abc'
  const onUploadComplete = jest.fn()

  beforeEach(() => {
    onUploadComplete.mockReset()
  })

  /**
   * Renders the component with default props.
   *
   * @returns The RTL render result.
   */
  function setup() {
    return render(<MaterialUpload courseId={courseId} onUploadComplete={onUploadComplete} />)
  }

  it('renders drop zone with browse text', () => {
    setup()

    expect(screen.getByText(/browse/i)).toBeInTheDocument()
    expect(screen.getByText(/drop a file/i)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /upload material/i })).not.toBeInTheDocument()
  })

  it('shows error for unsupported file type (text/plain)', async () => {
    setup()

    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    expect(input).not.toBeNull()

    // Use fireEvent.change to bypass the `accept` attribute which would
    // prevent userEvent.upload from firing onChange for unsupported types.
    const badFile = makeFile('notes.txt', 'text/plain')
    fireEvent.change(input, { target: { files: [badFile] } })

    expect(screen.getByText(/unsupported type/i)).toBeInTheDocument()

    // The upload button must NOT appear for an unsupported file.
    expect(screen.queryByRole('button', { name: /upload material/i })).not.toBeInTheDocument()
  })

  it('shows filename and Upload button for a valid PDF', async () => {
    const user = userEvent.setup()
    setup()

    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const pdfFile = makeFile('lecture.pdf', 'application/pdf')
    await user.upload(input, pdfFile)

    expect(screen.getByText('lecture.pdf')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /upload material/i })).toBeInTheDocument()
  })

  it('clicking ✕ clears the selected file', async () => {
    const user = userEvent.setup()
    setup()

    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const pdfFile = makeFile('lecture.pdf', 'application/pdf')
    await user.upload(input, pdfFile)

    // Confirm file is shown.
    expect(screen.getByText('lecture.pdf')).toBeInTheDocument()

    const removeBtn = screen.getByRole('button', { name: /remove file/i })
    await user.click(removeBtn)

    // File display and upload button should be gone; drop zone text returns.
    expect(screen.queryByText('lecture.pdf')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /upload material/i })).not.toBeInTheDocument()
    expect(screen.getByText(/browse/i)).toBeInTheDocument()
  })

  it('successful upload calls onUploadComplete with correct shape', async () => {
    const user = userEvent.setup()
    setup()

    const upstreamResponse = {
      job_id: 'job-123',
      material_id: 'mat-456',
      status: 'pending',
    }
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => upstreamResponse,
    })

    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const pdfFile = makeFile('slides.pdf', 'application/pdf')
    await user.upload(input, pdfFile)

    await user.click(screen.getByRole('button', { name: /upload material/i }))

    await waitFor(() => {
      expect(onUploadComplete).toHaveBeenCalledTimes(1)
    })

    const result: UploadedMaterial = onUploadComplete.mock.calls[0][0]
    expect(result.jobId).toBe('job-123')
    expect(result.materialId).toBe('mat-456')
    expect(result.filename).toBe('slides.pdf')
    expect(result.status).toBe('pending')
    expect(result.fileType).toBe('PDF')
    expect(typeof result.uploadedAt).toBe('string')

    // Verify the fetch was called with the correct URL.
    expect(mockFetch).toHaveBeenCalledWith(
      `/api/materials/upload?course_id=${encodeURIComponent(courseId)}`,
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('failed upload (non-ok response) shows error message', async () => {
    const user = userEvent.setup()
    setup()

    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: async () => ({ detail: 'Storage quota exceeded' }),
    })

    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    await user.upload(input, makeFile('big.pdf', 'application/pdf'))

    await user.click(screen.getByRole('button', { name: /upload material/i }))

    await waitFor(() => {
      expect(screen.getByText(/storage quota exceeded/i)).toBeInTheDocument()
    })

    expect(onUploadComplete).not.toHaveBeenCalled()
  })

  it('dropping an unsupported file shows the type error', () => {
    setup()

    const dropZone = screen.getByText(/browse/i).closest('div')!
    const badFile = makeFile('data.csv', 'text/csv')

    fireEvent.dragOver(dropZone, {
      dataTransfer: makeDataTransfer(badFile),
    })
    fireEvent.drop(dropZone, {
      dataTransfer: makeDataTransfer(badFile),
    })

    expect(screen.getByText(/unsupported type/i)).toBeInTheDocument()
  })
})
