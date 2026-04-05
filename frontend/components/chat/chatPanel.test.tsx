/**
 * @jest-environment jsdom
 *
 * Tests for the ChatPanel component.
 *
 * Covers: welcome message on mount, user message append, typing indicator
 * during streaming, error message on fetch failure, and welcome message role.
 */

import React from 'react'
import { render, screen, fireEvent, waitFor, act, cleanup } from '@testing-library/react'
import '@testing-library/jest-dom'
import { ReadableStream as NodeReadableStream } from 'stream/web'
import { TextEncoder as NodeTextEncoder } from 'util'

// ---------------------------------------------------------------------------
// Module mocks (must precede the component import)
// ---------------------------------------------------------------------------

jest.mock('./ChatMessage', () => ({
  __esModule: true,
  default: ({ message }: { message: { role: string; content: string } }) => (
    <div data-testid="chat-message" data-role={message.role}>
      {message.content}
    </div>
  ),
}))

jest.mock('./ChatInput', () => ({
  __esModule: true,
  default: ({
    value,
    onChange,
    onSubmit,
  }: {
    value: string
    onChange: (v: string) => void
    onSubmit: () => void
    disabled?: boolean
  }) => (
    <>
      <input data-testid="chat-input" value={value} onChange={(e) => onChange(e.target.value)} />
      <button data-testid="send-btn" onClick={onSubmit}>
        Send
      </button>
    </>
  ),
}))

jest.mock('./MoleculeBuilder', () => ({
  __esModule: true,
  default: () => <div data-testid="molecule-builder" />,
}))

// ---------------------------------------------------------------------------
// Component import (after mocks are registered)
// ---------------------------------------------------------------------------

import ChatPanel from './ChatPanel'

// jsdom doesn't implement scrollIntoView — stub it out.
// Also polyfill ReadableStream / TextEncoder for the Node jsdom environment.
beforeAll(() => {
  window.HTMLElement.prototype.scrollIntoView = jest.fn()
  if (typeof globalThis.ReadableStream === 'undefined') {
    globalThis.ReadableStream = NodeReadableStream as unknown as typeof globalThis.ReadableStream
  }
  if (typeof globalThis.TextEncoder === 'undefined') {
    globalThis.TextEncoder = NodeTextEncoder as unknown as typeof globalThis.TextEncoder
  }
})

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Builds a fetch mock that returns an immediately-readable SSE stream with the
 * given lines.  Each string in `sseLines` is emitted as a raw line chunk.
 */
function makeSseFetch(sseLines: string[], ok = true): jest.Mock {
  return jest.fn().mockResolvedValue({
    ok,
    status: ok ? 200 : 500,
    body: new ReadableStream({
      start(controller) {
        const encoder = new TextEncoder()
        for (const line of sseLines) {
          controller.enqueue(encoder.encode(line + '\n'))
        }
        controller.close()
      },
    }),
  })
}

/**
 * Creates a fetch mock whose response resolves only when `resolve()` is called,
 * allowing tests to observe the in-flight state.
 */
function makeHangingFetch(): { fetch: jest.Mock; resolve: () => void } {
  let resolveFn!: () => void
  const promise = new Promise<Response>((res) => {
    resolveFn = () =>
      res({
        ok: true,
        status: 200,
        body: new ReadableStream({
          start(controller) {
            controller.enqueue(new TextEncoder().encode('data: {"type":"delta","content":"Hi"}\n'))
            controller.close()
          },
        }),
      } as unknown as Response)
  })
  return { fetch: jest.fn().mockReturnValue(promise), resolve: resolveFn }
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

let consoleErrorSpy: jest.SpyInstance

beforeEach(() => {
  consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => undefined)
})

afterEach(() => {
  consoleErrorSpy.mockRestore()
  cleanup()
  jest.restoreAllMocks()
})

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('ChatPanel', () => {
  it('renders welcome message from Professor Nova on mount', () => {
    render(<ChatPanel />)

    const messages = screen.getAllByTestId('chat-message')
    expect(messages.length).toBeGreaterThanOrEqual(1)

    const welcomeMsg = messages[0]
    expect(welcomeMsg).toHaveTextContent('Professor Nova')
  })

  it('shows user message in message list after sending', async () => {
    global.fetch = makeSseFetch([
      'data: {"type":"delta","content":"Response text"}',
      'data: {"type":"done"}',
    ])

    render(<ChatPanel />)

    const input = screen.getByTestId('chat-input')
    const sendBtn = screen.getByTestId('send-btn')

    fireEvent.change(input, { target: { value: 'What is a covalent bond?' } })
    fireEvent.click(sendBtn)

    await waitFor(() => {
      const messages = screen.getAllByTestId('chat-message')
      const userMessages = messages.filter((el) => el.getAttribute('data-role') === 'user')
      expect(userMessages.length).toBeGreaterThanOrEqual(1)
      expect(userMessages[0]).toHaveTextContent('What is a covalent bond?')
    })
  })

  it('shows typing indicator (streaming dots) while fetch is in-flight', async () => {
    const { fetch: hangingFetch, resolve } = makeHangingFetch()
    global.fetch = hangingFetch

    render(<ChatPanel />)

    const input = screen.getByTestId('chat-input')
    const sendBtn = screen.getByTestId('send-btn')

    fireEvent.change(input, { target: { value: 'Tell me about benzene' } })

    await act(async () => {
      fireEvent.click(sendBtn)
    })

    // The animated bounce dots are rendered while isStreaming is true.
    // They share the animate-bounce class on their parent container siblings.
    const bouncingDots = document.querySelectorAll('.animate-bounce')
    expect(bouncingDots.length).toBeGreaterThan(0)

    // Resolve the stream so the component can settle.
    await act(async () => {
      resolve()
    })
  })

  it('shows error message in chat when fetch fails', async () => {
    global.fetch = jest.fn().mockRejectedValue(new Error('Network failure'))

    render(<ChatPanel />)

    const input = screen.getByTestId('chat-input')
    const sendBtn = screen.getByTestId('send-btn')

    fireEvent.change(input, { target: { value: 'This will fail' } })

    await act(async () => {
      fireEvent.click(sendBtn)
    })

    await waitFor(() => {
      const messages = screen.getAllByTestId('chat-message')
      const errorMessage = messages.find((el) =>
        el.textContent?.includes('Sorry, something went wrong'),
      )
      expect(errorMessage).toBeTruthy()
    })
  })

  it('welcome message has role assistant', () => {
    render(<ChatPanel />)

    const messages = screen.getAllByTestId('chat-message')
    expect(messages[0]).toHaveAttribute('data-role', 'assistant')
  })
})
