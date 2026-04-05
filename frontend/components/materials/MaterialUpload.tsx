'use client'

import React, { useRef, useState, DragEvent, ChangeEvent } from 'react'

const ACCEPTED_TYPES: Record<string, string> = {
  'application/pdf': 'PDF',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document': 'DOCX',
  'application/vnd.openxmlformats-officedocument.presentationml.presentation': 'PPTX',
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet': 'XLSX',
  'video/mp4': 'MP4',
  'video/quicktime': 'MOV',
  'audio/mpeg': 'MP3',
  'audio/wav': 'WAV',
  'image/jpeg': 'JPG',
  'image/png': 'PNG',
}

export interface UploadedMaterial {
  jobId: string
  materialId: string
  filename: string
  status: 'pending' | 'processing' | 'complete' | 'failed'
  fileType: string
  uploadedAt: string
}

interface Props {
  courseId: string
  onUploadComplete: (result: UploadedMaterial) => void
}

export default function MaterialUpload({ courseId, onUploadComplete }: Props) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [dragging, setDragging] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [selectedFile, setSelectedFile] = useState<File | null>(null)

  function handleDragOver(e: DragEvent<HTMLDivElement>) {
    e.preventDefault()
    setDragging(true)
  }

  function handleDragLeave(e: DragEvent<HTMLDivElement>) {
    e.preventDefault()
    setDragging(false)
  }

  function handleDrop(e: DragEvent<HTMLDivElement>) {
    e.preventDefault()
    setDragging(false)
    const file = e.dataTransfer.files[0]
    if (file) pickFile(file)
  }

  function handleInputChange(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (file) pickFile(file)
  }

  function pickFile(file: File) {
    setError(null)
    if (!ACCEPTED_TYPES[file.type]) {
      setError(`Unsupported type. Accepted: ${Object.values(ACCEPTED_TYPES).join(', ')}`)
      return
    }
    setSelectedFile(file)
  }

  function clearFile(e: React.MouseEvent) {
    e.stopPropagation()
    setSelectedFile(null)
    setError(null)
    if (inputRef.current) inputRef.current.value = ''
  }

  async function handleUpload() {
    if (!selectedFile) return
    setUploading(true)
    setError(null)

    try {
      const body = new FormData()
      body.append('file', selectedFile)

      const res = await fetch(`/api/materials/upload?course_id=${encodeURIComponent(courseId)}`, {
        method: 'POST',
        body,
      })

      const data: { job_id?: string; material_id?: string; status?: string; detail?: string } =
        await res.json().catch(() => ({ detail: res.statusText }))

      if (!res.ok) {
        setError(data.detail ?? 'Upload failed')
        return
      }

      onUploadComplete({
        jobId: data.job_id ?? crypto.randomUUID(),
        materialId: data.material_id ?? crypto.randomUUID(),
        filename: selectedFile.name,
        status: (data.status as UploadedMaterial['status']) ?? 'pending',
        fileType: ACCEPTED_TYPES[selectedFile.type] ?? selectedFile.type,
        uploadedAt: new Date().toISOString(),
      })
      setSelectedFile(null)
      if (inputRef.current) inputRef.current.value = ''
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="flex flex-col gap-2">
      {/* Drop zone */}
      <div
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        onClick={() => !selectedFile && inputRef.current?.click()}
        className={`
          relative border-2 border-dashed rounded-lg p-3 text-center transition-all select-none
          ${
            selectedFile
              ? 'border-neon-green/50 bg-neon-green/5 cursor-default'
              : dragging
                ? 'border-neon-blue bg-neon-blue/10 shadow-neon-blue-sm cursor-copy'
                : 'border-border-mid hover:border-neon-blue/50 hover:bg-neon-blue/5 cursor-pointer'
          }
        `}
      >
        <input
          ref={inputRef}
          type="file"
          accept={Object.keys(ACCEPTED_TYPES).join(',')}
          onChange={handleInputChange}
          className="hidden"
        />

        {selectedFile ? (
          <div className="flex items-center gap-2">
            <FileTypeEmoji type={ACCEPTED_TYPES[selectedFile.type] ?? '?'} />
            <div className="flex-1 min-w-0 text-left">
              <p className="text-xs text-text-bright font-medium truncate">{selectedFile.name}</p>
              <p className="text-[10px] text-text-dim">
                {ACCEPTED_TYPES[selectedFile.type]} · {formatBytes(selectedFile.size)}
              </p>
            </div>
            <button
              onClick={clearFile}
              className="text-text-dim hover:text-text-base text-xs flex-shrink-0 w-5 h-5 flex items-center justify-center rounded hover:bg-border-dim"
              aria-label="Remove file"
            >
              ✕
            </button>
          </div>
        ) : (
          <div className="flex flex-col items-center gap-1.5 py-2">
            <UploadIcon />
            <p className="text-xs text-text-dim">
              <span className="text-neon-blue">Browse</span> or drop a file
            </p>
            <p className="text-[10px] text-text-dim">
              PDF · DOCX · PPTX · MP4 · images · 500 MB max
            </p>
          </div>
        )}
      </div>

      {error && <p className="text-[10px] text-red-400 px-0.5">{error}</p>}

      {selectedFile && (
        <button
          onClick={handleUpload}
          disabled={uploading}
          className="w-full py-2 rounded-lg bg-neon-blue/10 border border-neon-blue/40 text-neon-blue text-xs font-medium hover:bg-neon-blue/20 hover:shadow-neon-blue-sm disabled:opacity-40 disabled:cursor-not-allowed transition-all flex items-center justify-center gap-2"
        >
          {uploading ? (
            <>
              <SpinnerIcon /> Uploading…
            </>
          ) : (
            'Upload Material'
          )}
        </button>
      )}
    </div>
  )
}

function FileTypeEmoji({ type }: { type: string }) {
  const map: Record<string, string> = {
    MP4: '🎬',
    MOV: '🎬',
    MP3: '🎵',
    WAV: '🎵',
    JPG: '🖼️',
    PNG: '🖼️',
    PDF: '📄',
  }
  return <span className="text-xl leading-none flex-shrink-0">{map[type] ?? '📝'}</span>
}

function UploadIcon() {
  return (
    <svg
      className="text-text-dim"
      width="22"
      height="22"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4" />
      <polyline points="17 8 12 3 7 8" />
      <line x1="12" y1="3" x2="12" y2="15" />
    </svg>
  )
}

function SpinnerIcon() {
  return (
    <svg
      className="animate-spin flex-shrink-0"
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
    >
      <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" />
    </svg>
  )
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
