/**
 * @jest-environment jsdom
 */

/**
 * Tests for UploadGuideStep — materials upload walkthrough.
 */

import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import UploadGuideStep from './UploadGuideStep'

describe('UploadGuideStep', () => {
  it('renders the three walkthrough steps', () => {
    render(<UploadGuideStep onNext={() => {}} />)
    expect(screen.getByText(/pick a subject/i)).toBeInTheDocument()
    expect(screen.getByText(/upload a file/i)).toBeInTheDocument()
    expect(screen.getByText(/ask questions/i)).toBeInTheDocument()
  })

  it('mentions supported file types', () => {
    render(<UploadGuideStep onNext={() => {}} />)
    const matches = screen.getAllByText(/pdf/i)
    expect(matches.length).toBeGreaterThan(0)
  })

  it('calls onNext when "Got it" is clicked', () => {
    const onNext = jest.fn()
    render(<UploadGuideStep onNext={onNext} />)
    fireEvent.click(screen.getByRole('button', { name: /got it/i }))
    expect(onNext).toHaveBeenCalledTimes(1)
  })
})
