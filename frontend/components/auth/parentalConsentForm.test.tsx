/**
 * @jest-environment jsdom
 *
 * Tests for ParentalConsentForm — collects guardian email and submits
 * a consent request.  Backend wiring is skeleton-only; tests verify the
 * form contract and error states.
 */
import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ParentalConsentForm from './ParentalConsentForm'

describe('ParentalConsentForm', () => {
  it('renders the guardian email input', () => {
    render(<ParentalConsentForm userId="user-123" onSuccess={jest.fn()} />)
    expect(screen.getByLabelText(/guardian email/i)).toBeInTheDocument()
  })

  it('renders a submit button', () => {
    render(<ParentalConsentForm userId="user-123" onSuccess={jest.fn()} />)
    expect(screen.getByRole('button', { name: /send consent request/i })).toBeInTheDocument()
  })

  it('shows validation error when submitted with empty email', async () => {
    render(<ParentalConsentForm userId="user-123" onSuccess={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /send consent request/i }))
    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
    })
  })

  it('shows validation error for malformed email', async () => {
    render(<ParentalConsentForm userId="user-123" onSuccess={jest.fn()} />)
    fireEvent.change(screen.getByLabelText(/guardian email/i), {
      target: { value: 'not-an-email' },
    })
    fireEvent.click(screen.getByRole('button', { name: /send consent request/i }))
    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
    })
  })

  it('calls onSuccess after a successful submission', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ message: 'consent email sent' }),
    }) as jest.Mock

    const onSuccess = jest.fn()
    render(<ParentalConsentForm userId="user-123" onSuccess={onSuccess} />)
    fireEvent.change(screen.getByLabelText(/guardian email/i), {
      target: { value: 'parent@example.com' },
    })
    fireEvent.click(screen.getByRole('button', { name: /send consent request/i }))
    await waitFor(() => {
      expect(onSuccess).toHaveBeenCalledTimes(1)
    })
  })

  it('shows server error when fetch fails', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: false,
      json: async () => ({ error: 'failed to send email' }),
    }) as jest.Mock

    render(<ParentalConsentForm userId="user-123" onSuccess={jest.fn()} />)
    fireEvent.change(screen.getByLabelText(/guardian email/i), {
      target: { value: 'parent@example.com' },
    })
    fireEvent.click(screen.getByRole('button', { name: /send consent request/i }))
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/failed to send email/i)
    })
  })
})
