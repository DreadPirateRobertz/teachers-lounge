/**
 * @jest-environment jsdom
 */

/**
 * Tests for WelcomeStep — introduction screen of the onboarding wizard.
 */

import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import WelcomeStep from './WelcomeStep'

describe('WelcomeStep', () => {
  it('displays the user display name in the greeting', () => {
    render(<WelcomeStep displayName="MathWizard" onNext={() => {}} />)
    expect(screen.getByText(/Welcome, MathWizard!/i)).toBeInTheDocument()
  })

  it('renders all feature bullet points', () => {
    render(<WelcomeStep displayName="Alice" onNext={() => {}} />)
    expect(screen.getByText(/adapts to your learning style/i)).toBeInTheDocument()
    expect(screen.getByText(/boss battles/i)).toBeInTheDocument()
    expect(screen.getByText(/upload any study material/i)).toBeInTheDocument()
    expect(screen.getByText(/quests, xp/i)).toBeInTheDocument()
  })

  it('calls onNext when the CTA button is clicked', () => {
    const onNext = jest.fn()
    render(<WelcomeStep displayName="Alice" onNext={onNext} />)
    fireEvent.click(screen.getByRole('button', { name: /let's go/i }))
    expect(onNext).toHaveBeenCalledTimes(1)
  })

  it('renders the CTA button', () => {
    render(<WelcomeStep displayName="Bob" onNext={() => {}} />)
    expect(screen.getByRole('button')).toBeInTheDocument()
  })
})
