/**
 * @jest-environment jsdom
 *
 * Tests for AgeGate — renders children for adults / users with guardian consent,
 * shows the consent prompt for unverified minors.
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import AgeGate from './AgeGate'

describe('AgeGate', () => {
  it('renders children when account type is standard', () => {
    render(
      <AgeGate accountType="standard" guardianConsentAt={null}>
        <span>protected content</span>
      </AgeGate>,
    )
    expect(screen.getByText('protected content')).toBeInTheDocument()
  })

  it('renders children when minor has guardian consent', () => {
    render(
      <AgeGate accountType="minor" guardianConsentAt="2026-01-15T10:00:00Z">
        <span>protected content</span>
      </AgeGate>,
    )
    expect(screen.getByText('protected content')).toBeInTheDocument()
  })

  it('shows consent prompt when minor has no guardian consent', () => {
    render(
      <AgeGate accountType="minor" guardianConsentAt={null}>
        <span>protected content</span>
      </AgeGate>,
    )
    expect(screen.queryByText('protected content')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: /parental consent required/i })).toBeInTheDocument()
  })

  it('consent prompt shows a link to the consent flow', () => {
    render(
      <AgeGate accountType="minor" guardianConsentAt={null}>
        <span>protected content</span>
      </AgeGate>,
    )
    expect(screen.getByRole('link', { name: /request consent/i })).toBeInTheDocument()
  })

  it('renders children when account type is undefined (unauthenticated guest)', () => {
    render(
      <AgeGate accountType={undefined} guardianConsentAt={null}>
        <span>guest content</span>
      </AgeGate>,
    )
    expect(screen.getByText('guest content')).toBeInTheDocument()
  })
})
