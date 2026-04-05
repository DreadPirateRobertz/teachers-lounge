/**
 * @jest-environment jsdom
 *
 * Tests for the MathBlock component.
 *
 * Covers: inline span vs block div rendering, aria-label format, and
 * katex.render being called after mount via the dynamic import path.
 */

import React from 'react'
import { render, screen, waitFor, act } from '@testing-library/react'
import '@testing-library/jest-dom'
import { cleanup } from '@testing-library/react'

// ---------------------------------------------------------------------------
// Mock katex before the component import so the dynamic import resolves to it.
// ---------------------------------------------------------------------------

jest.mock('katex', () => ({
  __esModule: true,
  default: { render: jest.fn() },
  render: jest.fn(),
}))

// ---------------------------------------------------------------------------
// Component import (after mock registration)
// ---------------------------------------------------------------------------

import MathBlock from './MathBlock'
import katex from 'katex'

const mockKatexRender = (katex as unknown as { render: jest.Mock }).render

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

beforeEach(() => {
  mockKatexRender.mockClear()
})

afterEach(() => {
  cleanup()
})

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('MathBlock', () => {
  it('renders a span in inline mode (block prop omitted / false)', () => {
    render(<MathBlock expression="x^2" />)
    // The component renders either a <span> or <div> at the root level.
    // In inline mode a span is used.
    const el = document.querySelector('span[aria-label]')
    expect(el).not.toBeNull()
  })

  it('renders a div in block mode (block=true)', () => {
    render(<MathBlock expression="\int_0^\infty e^{-x} dx" block />)
    const el = document.querySelector('div[aria-label]')
    expect(el).not.toBeNull()
  })

  it('has correct aria-label containing the expression in block mode', () => {
    const expr = 'E = mc^2'
    render(<MathBlock expression={expr} block />)
    const el = document.querySelector('[aria-label]')
    expect(el).toHaveAttribute('aria-label', `Math: ${expr}`)
  })

  it('calls katex.render after mount (flushes dynamic import)', async () => {
    render(<MathBlock expression="a^2 + b^2 = c^2" />)

    await act(async () => {
      // Yield to the microtask queue so the dynamic import promise resolves.
      await Promise.resolve()
    })

    await waitFor(() => {
      expect(mockKatexRender).toHaveBeenCalled()
    })
  })

  it('inline span has correct aria-label format "Math: ${expression}"', () => {
    const expr = 'y = mx + b'
    render(<MathBlock expression={expr} />)
    const el = document.querySelector('span[aria-label]')
    expect(el).toHaveAttribute('aria-label', `Math: ${expr}`)
  })
})
