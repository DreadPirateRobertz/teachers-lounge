'use client'

import { useCallback, useEffect, useRef, useState } from 'react'

// ── Types ─────────────────────────────────────────────────────────────────────

/** A bookmarked phrase in the TTS audio stream. */
export interface Phrase {
  /** Display text for the bookmark button. */
  text: string
  /** Offset from audio start in milliseconds. */
  startMs: number
}

interface Props {
  /** URL of the TTS audio file (mp3 / ogg). */
  audioUrl: string
  /** Optional accessible label shown above the player. */
  label?: string
  /** Key phrases that the student can jump to by clicking. */
  phrases?: Phrase[]
}

// ── Speed options ─────────────────────────────────────────────────────────────

const SPEEDS = [0.5, 0.75, 1, 1.25, 1.5, 2] as const
type Speed = (typeof SPEEDS)[number]

// ── Helpers ───────────────────────────────────────────────────────────────────

/**
 * Format seconds as mm:ss for the progress display.
 */
function formatTime(seconds: number): string {
  if (!isFinite(seconds)) return '0:00'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

// ── TtsPlayer ─────────────────────────────────────────────────────────────────

/**
 * Audio player for TTS explanations (auditory learner mode).
 *
 * Features:
 * - Play / pause with accessible button labels.
 * - Speed control: 0.5×, 0.75×, 1×, 1.25×, 1.5×, 2×.
 * - Progress bar showing elapsed / total time.
 * - Phrase bookmarks: clicking jumps to that timestamp.
 * - Error state displayed as an ARIA alert when the audio fails to load.
 */
export default function TtsPlayer({ audioUrl, label, phrases }: Props) {
  const audioRef = useRef<HTMLAudioElement>(null)
  const [playing, setPlaying] = useState(false)
  const [speed, setSpeed] = useState<Speed>(1)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [error, setError] = useState(false)

  // Sync duration once metadata loads
  useEffect(() => {
    const audio = audioRef.current
    if (!audio) return

    const onLoaded = () => setDuration(audio.duration)
    const onTimeUpdate = () => setCurrentTime(audio.currentTime)
    const onEnded = () => setPlaying(false)
    const onError = () => setError(true)

    audio.addEventListener('loadedmetadata', onLoaded)
    audio.addEventListener('timeupdate', onTimeUpdate)
    audio.addEventListener('ended', onEnded)
    audio.addEventListener('error', onError)

    return () => {
      audio.removeEventListener('loadedmetadata', onLoaded)
      audio.removeEventListener('timeupdate', onTimeUpdate)
      audio.removeEventListener('ended', onEnded)
      audio.removeEventListener('error', onError)
    }
  }, [])

  const togglePlay = useCallback(async () => {
    const audio = audioRef.current
    if (!audio) return
    if (playing) {
      audio.pause()
      setPlaying(false)
    } else {
      await audio.play()
      setPlaying(true)
    }
  }, [playing])

  const handleSpeedChange = useCallback((s: Speed) => {
    const audio = audioRef.current
    if (audio) audio.playbackRate = s
    setSpeed(s)
  }, [])

  const seekToPhrase = useCallback((startMs: number) => {
    const audio = audioRef.current
    if (!audio) return
    audio.currentTime = startMs / 1000
    setCurrentTime(startMs / 1000)
  }, [])

  const handleSeek = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const audio = audioRef.current
    const t = Number(e.target.value)
    if (audio) audio.currentTime = t
    setCurrentTime(t)
  }, [])

  const progress = duration > 0 ? (currentTime / duration) * 100 : 0

  return (
    <div
      className="flex flex-col gap-2.5 p-3 bg-bg-card border border-border-mid rounded-xl"
      aria-label={label ?? 'Audio explanation player'}
    >
      {/* Label */}
      {label && <p className="text-xs font-mono text-neon-blue truncate">{label}</p>}

      {/* Error alert */}
      {error && (
        <div
          role="alert"
          className="text-xs text-red-400 bg-red-950/30 border border-red-800/50 rounded px-2 py-1"
        >
          Failed to load audio. Please try again later.
        </div>
      )}

      {/* Hidden audio element */}
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <audio ref={audioRef} src={audioUrl} preload="metadata" />

      {/* Controls row */}
      <div className="flex items-center gap-2">
        {/* Play / Pause */}
        <button
          onClick={togglePlay}
          aria-label={playing ? 'Pause' : 'Play'}
          className="w-8 h-8 flex items-center justify-center rounded-full bg-neon-blue/20 border border-neon-blue/40 text-neon-blue hover:bg-neon-blue/30 transition-colors flex-shrink-0"
          disabled={error}
        >
          {playing ? (
            // Pause icon
            <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor" aria-hidden>
              <rect x="1" y="1" width="3.5" height="10" rx="1" />
              <rect x="7.5" y="1" width="3.5" height="10" rx="1" />
            </svg>
          ) : (
            // Play icon
            <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor" aria-hidden>
              <path d="M2 1.5 L10 6 L2 10.5 Z" />
            </svg>
          )}
        </button>

        {/* Progress bar */}
        <div className="flex-1 flex flex-col gap-0.5">
          <input
            type="range"
            min={0}
            max={duration || 100}
            value={currentTime}
            step={0.1}
            onChange={handleSeek}
            aria-label="Seek"
            className="w-full h-1.5 accent-[#00aaff] cursor-pointer"
            style={{
              background: `linear-gradient(to right, #00aaff ${progress}%, #1a1a4a ${progress}%)`,
            }}
          />
          <div className="flex justify-between text-[9px] font-mono text-text-dim">
            <span>{formatTime(currentTime)}</span>
            <span>{formatTime(duration)}</span>
          </div>
        </div>
      </div>

      {/* Speed selector */}
      <div className="flex items-center gap-1 flex-wrap" role="group" aria-label="Playback speed">
        <span className="text-[10px] font-mono text-text-dim mr-1">Speed:</span>
        {SPEEDS.map((s) => (
          <button
            key={s}
            onClick={() => handleSpeedChange(s)}
            aria-pressed={speed === s}
            className={`px-1.5 py-0.5 text-[10px] font-mono rounded border transition-colors ${
              speed === s
                ? 'bg-neon-blue/20 border-neon-blue text-neon-blue'
                : 'border-border-dim text-text-dim hover:border-border-mid'
            }`}
          >
            {s}×
          </button>
        ))}
      </div>

      {/* Phrase bookmarks */}
      {phrases && phrases.length > 0 && (
        <div>
          <p className="text-[10px] font-mono text-text-dim mb-1">Key phrases:</p>
          <ul aria-label="Key phrases" className="flex flex-col gap-1">
            {phrases.map((phrase, i) => (
              <li key={i}>
                <button
                  onClick={() => seekToPhrase(phrase.startMs)}
                  aria-label={phrase.text}
                  className="text-left text-[11px] text-text-base hover:text-neon-blue transition-colors leading-snug w-full"
                >
                  <span className="font-mono text-[9px] text-text-dim mr-1.5">
                    {formatTime(phrase.startMs / 1000)}
                  </span>
                  {phrase.text}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
