/**
 * @jest-environment jsdom
 *
 * Tests for the ParticleBurst component.
 *
 * Covers: children rendering, imperative triggerCorrect/triggerWrong ref calls,
 * empty-particles render, and particle div rendering when particles are present.
 */

import React, { useRef } from 'react'
import { render, screen, act } from '@testing-library/react'
import '@testing-library/jest-dom'
import { cleanup } from '@testing-library/react'

// ---------------------------------------------------------------------------
// Mock useParticleBurst before importing the component.
// ---------------------------------------------------------------------------

jest.mock('./useParticleBurst', () => ({
  useParticleBurst: jest.fn(() => ({
    particles: [],
    trigger: jest.fn(),
  })),
}))

import { useParticleBurst } from './useParticleBurst'

const mockUseParticleBurst = useParticleBurst as jest.Mock

// ---------------------------------------------------------------------------
// Component import (after mock registration)
// ---------------------------------------------------------------------------

import ParticleBurst, { type ParticleBurstHandle } from './ParticleBurst'

// ---------------------------------------------------------------------------
// Minimal Particle shape required by the component (life / maxLife for opacity)
// ---------------------------------------------------------------------------

const ONE_PARTICLE = {
  id: 1,
  x: 100,
  y: 200,
  vx: 0,
  vy: 0,
  life: 30,
  maxLife: 60,
  color: '#00ff88',
  size: 8,
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

beforeEach(() => {
  mockUseParticleBurst.mockReset()
  mockUseParticleBurst.mockReturnValue({
    particles: [],
    trigger: jest.fn(),
  })
})

afterEach(() => {
  cleanup()
})

// ---------------------------------------------------------------------------
// Helper — wrapper that exposes an imperative ref
// ---------------------------------------------------------------------------

/**
 * Renders ParticleBurst with a forwarded ref so tests can invoke the handle.
 * Returns the ref after rendering.
 */
function renderWithRef(children: React.ReactNode = <span>child content</span>) {
  let capturedRef!: React.RefObject<ParticleBurstHandle>

  function Wrapper() {
    const ref = useRef<ParticleBurstHandle>(null)
    capturedRef = ref
    return <ParticleBurst ref={ref}>{children}</ParticleBurst>
  }

  const result = render(<Wrapper />)
  return { ...result, ref: capturedRef }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('ParticleBurst', () => {
  it('renders children content', () => {
    renderWithRef(<span data-testid="inner-child">Hello</span>)
    expect(screen.getByTestId('inner-child')).toBeInTheDocument()
    expect(screen.getByTestId('inner-child')).toHaveTextContent('Hello')
  })

  it('triggerCorrect on the ref calls the hook trigger with "correct"', () => {
    const mockTrigger = jest.fn()
    mockUseParticleBurst.mockReturnValue({ particles: [], trigger: mockTrigger })

    const { ref } = renderWithRef()

    act(() => {
      ref.current?.triggerCorrect({ x: 100, y: 200 })
    })

    expect(mockTrigger).toHaveBeenCalledTimes(1)
    expect(mockTrigger).toHaveBeenCalledWith('correct', { x: 100, y: 200 })
  })

  it('triggerWrong on the ref calls the hook trigger with "wrong"', () => {
    const mockTrigger = jest.fn()
    mockUseParticleBurst.mockReturnValue({ particles: [], trigger: mockTrigger })

    const { ref } = renderWithRef()

    act(() => {
      ref.current?.triggerWrong({ x: 50, y: 75 })
    })

    expect(mockTrigger).toHaveBeenCalledTimes(1)
    expect(mockTrigger).toHaveBeenCalledWith('wrong', { x: 50, y: 75 })
  })

  it('renders without crashing when particles array is empty', () => {
    mockUseParticleBurst.mockReturnValue({ particles: [], trigger: jest.fn() })

    const { container } = renderWithRef()
    // The outer relative-positioned wrapper should still be present.
    const wrapper = container.querySelector('[style*="position: relative"]')
    expect(wrapper).not.toBeNull()
  })

  it('renders particle elements when particles are returned from the hook', () => {
    mockUseParticleBurst.mockReturnValue({
      particles: [ONE_PARTICLE],
      trigger: jest.fn(),
    })

    const { container } = renderWithRef()

    // The particle overlay div contains aria-hidden="true", and each particle
    // is a child div inside it.  The component maps particles → absolute divs.
    const overlay = container.querySelector('[aria-hidden="true"]')
    expect(overlay).not.toBeNull()

    // There should be exactly one child div for the one particle.
    const particleDivs = overlay?.querySelectorAll('div')
    expect(particleDivs?.length).toBe(1)

    const particleEl = particleDivs![0] as HTMLElement
    // jsdom normalises hex to rgb(), so compare via the style attribute string
    expect(particleEl.getAttribute('style')).toContain(ONE_PARTICLE.color)
    expect(particleEl.style.left).toBe(`${ONE_PARTICLE.x}px`)
    expect(particleEl.style.top).toBe(`${ONE_PARTICLE.y}px`)
  })
})
