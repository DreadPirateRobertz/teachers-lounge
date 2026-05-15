/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import 'jest-canvas-mock'
import VictoryParticles from './VictoryParticles'

/**
 * Stub matchMedia so we can flip between reduced-motion on/off per test.
 */
function setReducedMotion(matches: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    configurable: true,
    value: jest.fn().mockImplementation((query: string) => ({
      matches: query.includes('reduce') ? matches : false,
      media: query,
      onchange: null,
      addListener: jest.fn(),
      removeListener: jest.fn(),
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
      dispatchEvent: jest.fn(),
    })),
  })
}

describe('VictoryParticles', () => {
  let rafSpy: jest.SpyInstance
  let cancelSpy: jest.SpyInstance

  beforeEach(() => {
    setReducedMotion(false)
    let callCount = 0
    rafSpy = jest.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
      callCount += 1
      // Drive exactly one frame to exercise the paint path; ignore subsequent
      // schedule calls so the loop terminates instead of recursing forever.
      if (callCount === 1) cb(performance.now() + 16)
      return callCount as unknown as number
    })
    cancelSpy = jest.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => {})
  })

  afterEach(() => {
    rafSpy.mockRestore()
    cancelSpy.mockRestore()
  })

  it('renders a decorative canvas when active', () => {
    render(<VictoryParticles active={true} />)
    const canvas = screen.getByTestId('victory-particles')
    expect(canvas.tagName).toBe('CANVAS')
    expect(canvas).toHaveAttribute('aria-hidden', 'true')
  })

  it('renders nothing when inactive', () => {
    render(<VictoryParticles active={false} />)
    expect(screen.queryByTestId('victory-particles')).toBeNull()
  })

  it('renders nothing when prefers-reduced-motion is set', () => {
    setReducedMotion(true)
    render(<VictoryParticles active={true} />)
    expect(screen.queryByTestId('victory-particles')).toBeNull()
  })

  it('schedules an animation frame on mount and cancels on unmount', () => {
    const { unmount } = render(<VictoryParticles active={true} />)
    expect(rafSpy).toHaveBeenCalled()
    unmount()
    expect(cancelSpy).toHaveBeenCalled()
  })
})
