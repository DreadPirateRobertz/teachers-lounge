/**
 * @jest-environment jsdom
 *
 * Unit tests for TtsPlayer — the auditory-mode audio player.
 * Browser AudioContext and HTMLMediaElement are mocked so tests run in jsdom.
 */
import React from 'react'
import { render, screen, fireEvent, act } from '@testing-library/react'
import TtsPlayer, { type Phrase } from './TtsPlayer'

// ── HTMLMediaElement stub ─────────────────────────────────────────────────────
// jsdom does not implement HTMLMediaElement playback; stub the methods.

beforeAll(() => {
  Object.defineProperty(HTMLMediaElement.prototype, 'play', {
    configurable: true,
    value: jest.fn().mockResolvedValue(undefined),
  })
  Object.defineProperty(HTMLMediaElement.prototype, 'pause', {
    configurable: true,
    value: jest.fn(),
  })
  Object.defineProperty(HTMLMediaElement.prototype, 'load', {
    configurable: true,
    value: jest.fn(),
  })
  // duration must be writable so we can simulate a loaded track
  Object.defineProperty(HTMLMediaElement.prototype, 'duration', {
    configurable: true,
    writable: true,
    value: 120,
  })
  Object.defineProperty(HTMLMediaElement.prototype, 'currentTime', {
    configurable: true,
    writable: true,
    value: 0,
  })
  Object.defineProperty(HTMLMediaElement.prototype, 'playbackRate', {
    configurable: true,
    writable: true,
    value: 1,
  })
  Object.defineProperty(HTMLMediaElement.prototype, 'readyState', {
    configurable: true,
    writable: true,
    value: 4, // HAVE_ENOUGH_DATA
  })
})

// ── Fixtures ──────────────────────────────────────────────────────────────────

const AUDIO_URL = 'https://tts.example.com/audio/abc123.mp3'

const PHRASES: Phrase[] = [
  { text: 'Plants use sunlight to synthesize food.', startMs: 0 },
  { text: 'This process is called photosynthesis.', startMs: 5200 },
  { text: 'The main inputs are CO₂ and H₂O.', startMs: 11800 },
]

// ── Rendering ─────────────────────────────────────────────────────────────────

describe('TtsPlayer — rendering', () => {
  it('renders without crashing', () => {
    render(<TtsPlayer audioUrl={AUDIO_URL} />)
  })

  it('shows play button initially', () => {
    render(<TtsPlayer audioUrl={AUDIO_URL} />)
    expect(screen.getByRole('button', { name: /play/i })).toBeInTheDocument()
  })

  it('renders an accessible label when provided', () => {
    render(<TtsPlayer audioUrl={AUDIO_URL} label="Photosynthesis explanation" />)
    expect(screen.getByText('Photosynthesis explanation')).toBeInTheDocument()
  })

  it('renders phrase bookmarks when phrases are provided', () => {
    render(<TtsPlayer audioUrl={AUDIO_URL} phrases={PHRASES} />)
    expect(screen.getByText('Plants use sunlight to synthesize food.')).toBeInTheDocument()
    expect(screen.getByText('This process is called photosynthesis.')).toBeInTheDocument()
  })

  it('renders all speed options', () => {
    render(<TtsPlayer audioUrl={AUDIO_URL} />)
    // Speed buttons: 0.5×, 0.75×, 1×, 1.25×, 1.5×, 2×
    expect(screen.getByRole('button', { name: '0.5×' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '1×' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '2×' })).toBeInTheDocument()
  })
})

// ── Playback controls ─────────────────────────────────────────────────────────

describe('TtsPlayer — playback controls', () => {
  it('calls play() on the audio element when play button is clicked', async () => {
    render(<TtsPlayer audioUrl={AUDIO_URL} />)
    const playBtn = screen.getByRole('button', { name: /play/i })
    await act(async () => {
      fireEvent.click(playBtn)
    })
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalled()
  })

  it('shows pause button after play is clicked', async () => {
    render(<TtsPlayer audioUrl={AUDIO_URL} />)
    const playBtn = screen.getByRole('button', { name: /play/i })
    await act(async () => {
      fireEvent.click(playBtn)
    })
    // After clicking play, UI should switch to pause
    expect(screen.getByRole('button', { name: /pause/i })).toBeInTheDocument()
  })

  it('calls pause() on the audio element when pause button is clicked', async () => {
    render(<TtsPlayer audioUrl={AUDIO_URL} />)
    const playBtn = screen.getByRole('button', { name: /play/i })
    await act(async () => {
      fireEvent.click(playBtn)
    })
    const pauseBtn = screen.getByRole('button', { name: /pause/i })
    await act(async () => {
      fireEvent.click(pauseBtn)
    })
    expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled()
  })
})

// ── Speed control ─────────────────────────────────────────────────────────────

describe('TtsPlayer — speed control', () => {
  it('highlights 1× speed by default (aria-pressed)', () => {
    render(<TtsPlayer audioUrl={AUDIO_URL} />)
    const btn = screen.getByRole('button', { name: '1×' })
    expect(btn).toHaveAttribute('aria-pressed', 'true')
  })

  it('sets aria-pressed on selected speed button', () => {
    render(<TtsPlayer audioUrl={AUDIO_URL} />)
    const btn2x = screen.getByRole('button', { name: '2×' })
    fireEvent.click(btn2x)
    expect(btn2x).toHaveAttribute('aria-pressed', 'true')
  })

  it('removes aria-pressed from previously selected speed', () => {
    render(<TtsPlayer audioUrl={AUDIO_URL} />)
    const btn1x = screen.getByRole('button', { name: '1×' })
    const btn2x = screen.getByRole('button', { name: '2×' })
    fireEvent.click(btn2x)
    expect(btn1x).toHaveAttribute('aria-pressed', 'false')
  })

  it('sets audio.playbackRate when speed is changed', () => {
    const { container } = render(<TtsPlayer audioUrl={AUDIO_URL} />)
    const audio = container.querySelector('audio') as HTMLAudioElement
    const btn15x = screen.getByRole('button', { name: '1.5×' })
    fireEvent.click(btn15x)
    expect(audio.playbackRate).toBe(1.5)
  })
})

// ── Phrase bookmarks ──────────────────────────────────────────────────────────

describe('TtsPlayer — phrase bookmarks', () => {
  it('seeks audio to phrase startMs / 1000 when bookmark is clicked', () => {
    const { container } = render(<TtsPlayer audioUrl={AUDIO_URL} phrases={PHRASES} />)
    const audio = container.querySelector('audio') as HTMLAudioElement

    const phrase2Btn = screen.getByRole('button', { name: /This process is called photosynthesis/ })
    fireEvent.click(phrase2Btn)

    // 5200ms → 5.2s
    expect(audio.currentTime).toBeCloseTo(5.2)
  })

  it('renders no bookmark list when phrases prop is absent', () => {
    render(<TtsPlayer audioUrl={AUDIO_URL} />)
    expect(screen.queryByRole('list', { name: /key phrases/i })).toBeNull()
  })
})

// ── Error state ───────────────────────────────────────────────────────────────

describe('TtsPlayer — error state', () => {
  it('shows an error message when the audio element fires an error event', () => {
    const { container } = render(<TtsPlayer audioUrl={AUDIO_URL} />)
    const audio = container.querySelector('audio') as HTMLAudioElement
    act(() => {
      fireEvent.error(audio)
    })
    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByText(/failed to load audio/i)).toBeInTheDocument()
  })
})
