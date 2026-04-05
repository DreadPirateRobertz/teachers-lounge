'use client'

import React from 'react'
import type { UploadedMaterial } from './MaterialUpload'

const STATUS_CONFIG = {
  pending: {
    label: 'Pending',
    color: 'text-yellow-400',
    border: 'border-yellow-400/30',
    bg: 'bg-yellow-400/10',
    dot: 'bg-yellow-400',
    pulse: true,
  },
  processing: {
    label: 'Processing',
    color: 'text-neon-blue',
    border: 'border-neon-blue/30',
    bg: 'bg-neon-blue/10',
    dot: 'bg-neon-blue',
    pulse: true,
  },
  complete: {
    label: 'Ready',
    color: 'text-neon-green',
    border: 'border-neon-green/30',
    bg: 'bg-neon-green/10',
    dot: 'bg-neon-green',
    pulse: false,
  },
  failed: {
    label: 'Failed',
    color: 'text-red-400',
    border: 'border-red-400/30',
    bg: 'bg-red-400/10',
    dot: 'bg-red-400',
    pulse: false,
  },
} as const

const FILE_EMOJI: Record<string, string> = {
  MP4: '🎬',
  MOV: '🎬',
  MP3: '🎵',
  WAV: '🎵',
  JPG: '🖼️',
  PNG: '🖼️',
  PDF: '📄',
  DOCX: '📝',
  PPTX: '📊',
  XLSX: '📊',
}

interface Props {
  materials: UploadedMaterial[]
}

export default function MaterialLibrary({ materials }: Props) {
  if (materials.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-8 gap-2 text-center">
        <span className="text-3xl">📚</span>
        <p className="text-xs text-text-dim">No materials yet</p>
        <p className="text-[10px] text-text-dim">Upload a file above to get started</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-1.5">
      {materials.map((m) => (
        <MaterialRow key={m.jobId} material={m} />
      ))}
    </div>
  )
}

function MaterialRow({ material }: { material: UploadedMaterial }) {
  const cfg = STATUS_CONFIG[material.status]

  return (
    <div className="flex items-center gap-2 bg-bg-card border border-border-dim rounded p-2 animate-fade-in">
      <span className="text-base leading-none flex-shrink-0">
        {FILE_EMOJI[material.fileType] ?? '📄'}
      </span>
      <div className="flex-1 min-w-0">
        <p className="text-xs text-text-base font-medium truncate">{material.filename}</p>
        <p className="text-[10px] text-text-dim">
          {material.fileType} · {formatRelative(material.uploadedAt)}
        </p>
      </div>
      <StatusBadge cfg={cfg} />
    </div>
  )
}

type StatusCfg = (typeof STATUS_CONFIG)[keyof typeof STATUS_CONFIG]

function StatusBadge({ cfg }: { cfg: StatusCfg }) {
  return (
    <div
      className={`flex items-center gap-1 px-1.5 py-0.5 rounded border text-[10px] font-medium flex-shrink-0 ${cfg.bg} ${cfg.border} ${cfg.color}`}
    >
      <span
        className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${cfg.dot} ${cfg.pulse ? 'animate-pulse-slow' : ''}`}
      />
      {cfg.label}
    </div>
  )
}

function formatRelative(iso: string): string {
  const s = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (s < 60) return 'just now'
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}
