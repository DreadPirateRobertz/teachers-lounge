/**
 * @jest-environment jsdom
 */

/**
 * Tests for ReadyStep — wizard completion screen.
 */

import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import ReadyStep from './ReadyStep'

describe('ReadyStep', () => {
  it('shows the user display name', () => {
    render(<ReadyStep avatarEmoji="🧙" displayName="MathWizard" onDone={() => {}} />)
    expect(screen.getByText(/You're all set, MathWizard!/i)).toBeInTheDocument()
  })

  it('shows the avatar emoji', () => {
    render(<ReadyStep avatarEmoji="🦊" displayName="FoxUser" onDone={() => {}} />)
    // The emoji appears in the avatar display
    expect(screen.getAllByText('🦊').length).toBeGreaterThan(0)
  })

  it('lists completion checkmarks', () => {
    render(<ReadyStep avatarEmoji="🎓" displayName="Alice" onDone={() => {}} />)
    expect(screen.getByText(/character created/i)).toBeInTheDocument()
    expect(screen.getByText(/free trial active/i)).toBeInTheDocument()
  })

  it('calls onDone when "Start learning" is clicked', () => {
    const onDone = jest.fn()
    render(<ReadyStep avatarEmoji="🎓" displayName="Bob" onDone={onDone} />)
    fireEvent.click(screen.getByRole('button', { name: /start learning/i }))
    expect(onDone).toHaveBeenCalledTimes(1)
  })
})
