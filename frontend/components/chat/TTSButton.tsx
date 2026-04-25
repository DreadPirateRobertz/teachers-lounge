'use client'

import { useCallback, useEffect, useRef, useState } from 'react'

/** Backend voice options exposed to the frontend. */
export type TtsVoice = 'nova' | 'sage' | 'amber'

interface Props {
  /** The full message text to synthesize. Trimmed and capped server-side. */
  text: string
  /** Voice profile for the TTS request. Defaults to "nova". */
  voice?: TtsVoice
}

type Status = 'idle' | 'loading' | 'playing' | 'error'

/** Cap on the request payload to keep TTS requests cheap. */
const MAX_TTS_CHARS = 1500

/**
 * On-demand TTS button rendered next to every tutor message.
 *
 * Click → POST `/api/tts/speak` with `{ text, voice }`, receive an audio
 * blob, and play it through a hidden `HTMLAudioElement`. The button cycles
 * through idle → loading → playing states; clicking while playing pauses.
 *
 * The fetched blob URL is cached on the component instance so subsequent
 * plays of the same message do not re-call the backend.
 */
export default function TTSButton({ text, voice = 'nova' }: Props) {
  const [status, setStatus] = useState<Status>('idle')
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const blobUrlRef = useRef<string | null>(null)

  // Revoke the blob URL on unmount to avoid memory leaks.
  useEffect(() => {
    return () => {
      if (blobUrlRef.current) {
        URL.revokeObjectURL(blobUrlRef.current)
        blobUrlRef.current = null
      }
      audioRef.current?.pause()
      audioRef.current = null
    }
  }, [])

  const handleClick = useCallback(async () => {
    // Pause if already playing.
    if (status === 'playing' && audioRef.current) {
      audioRef.current.pause()
      setStatus('idle')
      return
    }
    if (status === 'loading') return

    // Replay cached blob if available.
    if (blobUrlRef.current && audioRef.current) {
      audioRef.current.currentTime = 0
      try {
        await audioRef.current.play()
        setStatus('playing')
      } catch {
        setStatus('error')
      }
      return
    }

    setStatus('loading')
    try {
      const res = await fetch('/api/tts/speak', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: text.slice(0, MAX_TTS_CHARS), voice }),
      })
      if (!res.ok) throw new Error(`TTS failed: ${res.status}`)
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      blobUrlRef.current = url
      const audio = new Audio(url)
      audio.addEventListener('ended', () => setStatus('idle'))
      audio.addEventListener('error', () => setStatus('error'))
      audioRef.current = audio
      await audio.play()
      setStatus('playing')
    } catch {
      setStatus('error')
    }
  }, [status, text, voice])

  const labelByStatus: Record<Status, string> = {
    idle: 'Listen to message',
    loading: 'Loading audio',
    playing: 'Pause audio',
    error: 'Audio failed — retry',
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      data-testid="tts-button"
      data-status={status}
      aria-label={labelByStatus[status]}
      disabled={status === 'loading'}
      className={`inline-flex items-center justify-center w-6 h-6 rounded-full border text-xs transition-colors disabled:cursor-wait ${
        status === 'error'
          ? 'border-red-700/40 bg-red-950/30 text-red-400'
          : status === 'playing'
            ? 'border-neon-pink/40 bg-neon-pink/10 text-neon-pink'
            : 'border-neon-blue/30 bg-neon-blue/10 text-neon-blue hover:bg-neon-blue/20'
      }`}
    >
      <span aria-hidden>
        {status === 'loading'
          ? '⏳'
          : status === 'playing'
            ? '⏸'
            : status === 'error'
              ? '⚠️'
              : '🔊'}
      </span>
    </button>
  )
}
