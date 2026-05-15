/**
 * @jest-environment jsdom
 *
 * Unit tests for TTSButton — on-demand speaker for tutor messages (tl-h9e).
 *
 * fetch and the `Audio` constructor are mocked so the component exercises
 * its full state machine (idle → loading → playing → idle/error) without
 * touching a real network or audio device.
 */
import React from 'react'
import { render, screen, act, fireEvent } from '@testing-library/react'
import TTSButton from './TTSButton'

interface MockAudio {
  play: jest.Mock<Promise<void>, []>
  pause: jest.Mock<void, []>
  addEventListener: jest.Mock<void, [string, () => void]>
  currentTime: number
  _listeners: Record<string, () => void>
}

const audioInstances: MockAudio[] = []
let originalAudio: typeof Audio
let originalCreateObjectURL: typeof URL.createObjectURL
let originalRevokeObjectURL: typeof URL.revokeObjectURL
let originalFetch: typeof fetch | undefined

beforeEach(() => {
  audioInstances.length = 0
  originalAudio = global.Audio
  originalCreateObjectURL = URL.createObjectURL
  originalRevokeObjectURL = URL.revokeObjectURL
  originalFetch = global.fetch

  global.Audio = jest.fn().mockImplementation(() => {
    const listeners: Record<string, () => void> = {}
    const inst: MockAudio = {
      play: jest.fn().mockResolvedValue(undefined),
      pause: jest.fn(),
      addEventListener: jest.fn((evt: string, cb: () => void) => {
        listeners[evt] = cb
      }),
      currentTime: 0,
      _listeners: listeners,
    }
    audioInstances.push(inst)
    return inst as unknown as HTMLAudioElement
  }) as unknown as typeof Audio

  URL.createObjectURL = jest.fn().mockReturnValue('blob:mock-url')
  URL.revokeObjectURL = jest.fn()
})

afterEach(() => {
  global.Audio = originalAudio
  URL.createObjectURL = originalCreateObjectURL
  URL.revokeObjectURL = originalRevokeObjectURL
  if (originalFetch) global.fetch = originalFetch
  else delete (global as { fetch?: typeof fetch }).fetch
  jest.restoreAllMocks()
})

function mockFetchSuccess() {
  const fetchMock = jest.fn().mockResolvedValue({
    ok: true,
    blob: jest.fn().mockResolvedValue(new Blob(['audio'], { type: 'audio/mpeg' })),
  } as unknown as Response)
  global.fetch = fetchMock as unknown as typeof fetch
  return fetchMock
}

describe('TTSButton', () => {
  it('renders idle state with accessible label', () => {
    render(<TTSButton text="Hello." />)
    const btn = screen.getByTestId('tts-button')
    expect(btn).toHaveAttribute('data-status', 'idle')
    expect(btn).toHaveAttribute('aria-label', 'Listen to message')
    expect(btn).not.toBeDisabled()
  })

  it('clicking idle posts to /api/tts/speak with text and default voice', async () => {
    const fetchMock = mockFetchSuccess()
    render(<TTSButton text="Read this aloud." />)

    await act(async () => {
      fireEvent.click(screen.getByTestId('tts-button'))
    })

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/tts/speak')
    expect((init as RequestInit).method).toBe('POST')
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({
      text: 'Read this aloud.',
      voice: 'nova',
    })
  })

  it('forwards a custom voice prop', async () => {
    const fetchMock = mockFetchSuccess()
    render(<TTSButton text="hi" voice="sage" />)
    await act(async () => {
      fireEvent.click(screen.getByTestId('tts-button'))
    })
    const init = fetchMock.mock.calls[0][1] as RequestInit
    expect(JSON.parse(init.body as string).voice).toBe('sage')
  })

  it('enters playing state after a successful fetch + play', async () => {
    mockFetchSuccess()
    render(<TTSButton text="Hello." />)
    await act(async () => {
      fireEvent.click(screen.getByTestId('tts-button'))
    })
    const btn = screen.getByTestId('tts-button')
    expect(btn).toHaveAttribute('data-status', 'playing')
    expect(btn).toHaveAttribute('aria-label', 'Pause audio')
    expect(audioInstances).toHaveLength(1)
    expect(audioInstances[0].play).toHaveBeenCalled()
  })

  it('clicking again while playing pauses and returns to idle', async () => {
    mockFetchSuccess()
    render(<TTSButton text="Hello." />)
    await act(async () => {
      fireEvent.click(screen.getByTestId('tts-button'))
    })
    await act(async () => {
      fireEvent.click(screen.getByTestId('tts-button'))
    })
    expect(audioInstances[0].pause).toHaveBeenCalled()
    expect(screen.getByTestId('tts-button')).toHaveAttribute('data-status', 'idle')
  })

  it('replays cached blob on second click without re-fetching', async () => {
    const fetchMock = mockFetchSuccess()
    render(<TTSButton text="Hello." />)

    // First click: fetch + play
    await act(async () => {
      fireEvent.click(screen.getByTestId('tts-button'))
    })
    expect(fetchMock).toHaveBeenCalledTimes(1)

    // Pause
    await act(async () => {
      fireEvent.click(screen.getByTestId('tts-button'))
    })

    // Replay: cached, no second fetch
    await act(async () => {
      fireEvent.click(screen.getByTestId('tts-button'))
    })
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(audioInstances[0].play).toHaveBeenCalledTimes(2)
  })

  it('shows error state when fetch fails', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: false,
      status: 503,
    } as unknown as Response) as unknown as typeof fetch

    render(<TTSButton text="Hello." />)
    await act(async () => {
      fireEvent.click(screen.getByTestId('tts-button'))
    })
    const btn = screen.getByTestId('tts-button')
    expect(btn).toHaveAttribute('data-status', 'error')
    expect(btn).toHaveAttribute('aria-label', 'Audio failed — retry')
  })

  it('returns to idle when the audio ends', async () => {
    mockFetchSuccess()
    render(<TTSButton text="Hello." />)
    await act(async () => {
      fireEvent.click(screen.getByTestId('tts-button'))
    })
    expect(screen.getByTestId('tts-button')).toHaveAttribute('data-status', 'playing')

    await act(async () => {
      audioInstances[0]._listeners.ended?.()
    })
    expect(screen.getByTestId('tts-button')).toHaveAttribute('data-status', 'idle')
  })

  it('truncates text payload to 1500 chars', async () => {
    const fetchMock = mockFetchSuccess()
    const longText = 'x'.repeat(2000)
    render(<TTSButton text={longText} />)
    await act(async () => {
      fireEvent.click(screen.getByTestId('tts-button'))
    })
    const init = fetchMock.mock.calls[0][1] as RequestInit
    const body = JSON.parse(init.body as string)
    expect(body.text).toHaveLength(1500)
  })

  it('revokes the blob URL on unmount', async () => {
    mockFetchSuccess()
    const { unmount } = render(<TTSButton text="Hello." />)
    await act(async () => {
      fireEvent.click(screen.getByTestId('tts-button'))
    })
    unmount()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-url')
  })
})
