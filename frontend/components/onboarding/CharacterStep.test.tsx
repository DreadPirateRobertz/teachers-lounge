/**
 * @jest-environment jsdom
 */

/**
 * Tests for CharacterStep — avatar picker and display name editor.
 */

import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import CharacterStep from './CharacterStep'

const defaultProps = {
  displayName: 'Scholar',
  avatarEmoji: '🎓',
  onNext: jest.fn(),
}

describe('CharacterStep', () => {
  beforeEach(() => {
    defaultProps.onNext.mockClear()
  })

  it('renders the display name input pre-filled', () => {
    render(<CharacterStep {...defaultProps} />)
    expect(screen.getByLabelText(/display name/i)).toHaveValue('Scholar')
  })

  it('renders avatar emoji options', () => {
    render(<CharacterStep {...defaultProps} />)
    // Spot-check a few emojis in the picker
    const wizardButton = screen.getByRole('button', { name: /select avatar 🧙/i })
    expect(wizardButton).toBeInTheDocument()
  })

  it('updates the avatar preview when an emoji is selected', () => {
    render(<CharacterStep {...defaultProps} />)
    fireEvent.click(screen.getByRole('button', { name: /select avatar 🧙/i }))
    // The selected button should be marked as pressed
    expect(screen.getByRole('button', { name: /select avatar 🧙/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
  })

  it('calls onNext with name and emoji on form submit', async () => {
    const onNext = jest.fn().mockResolvedValue(undefined)
    render(<CharacterStep {...defaultProps} onNext={onNext} />)

    const nameInput = screen.getByLabelText(/display name/i)
    fireEvent.change(nameInput, { target: { value: 'NewName' } })
    fireEvent.click(screen.getByRole('button', { name: /select avatar 🤖/i }))
    fireEvent.click(screen.getByRole('button', { name: /looks good/i }))

    await waitFor(() => {
      expect(onNext).toHaveBeenCalledWith('NewName', '🤖')
    })
  })

  it('shows an error when display name is empty', async () => {
    render(<CharacterStep {...defaultProps} />)
    const nameInput = screen.getByLabelText(/display name/i)
    fireEvent.change(nameInput, { target: { value: '' } })
    fireEvent.click(screen.getByRole('button', { name: /looks good/i }))
    expect(await screen.findByText(/display name is required/i)).toBeInTheDocument()
  })

  it('shows an error when onNext rejects', async () => {
    const onNext = jest.fn().mockRejectedValue(new Error('network error'))
    render(<CharacterStep {...defaultProps} onNext={onNext} />)
    fireEvent.click(screen.getByRole('button', { name: /looks good/i }))
    expect(await screen.findByText(/failed to save/i)).toBeInTheDocument()
  })

  it('disables the submit button while saving', async () => {
    let resolve!: () => void
    const onNext = jest.fn(
      () =>
        new Promise<void>((r) => {
          resolve = r
        }),
    )
    render(<CharacterStep {...defaultProps} onNext={onNext} />)
    fireEvent.click(screen.getByRole('button', { name: /looks good/i }))
    expect(screen.getByRole('button', { name: /saving/i })).toBeDisabled()
    resolve()
  })
})
