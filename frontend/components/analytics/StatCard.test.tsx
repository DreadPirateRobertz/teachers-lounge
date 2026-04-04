/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import StatCard from './StatCard'

describe('StatCard', () => {
  it('renders label and value', () => {
    render(<StatCard label="Total Messages" value={42} />)
    expect(screen.getByText('Total Messages')).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
  })

  it('renders optional sub text when provided', () => {
    render(<StatCard label="XP" value={1200} sub="+80 today" />)
    expect(screen.getByText('+80 today')).toBeInTheDocument()
  })

  it('does not render sub text when omitted', () => {
    render(<StatCard label="XP" value={1200} />)
    expect(screen.queryByText(/today/)).not.toBeInTheDocument()
  })

  it('renders string value', () => {
    render(<StatCard label="Rank" value="Gold" />)
    expect(screen.getByText('Gold')).toBeInTheDocument()
  })

  it('applies blue color classes by default', () => {
    const { container } = render(<StatCard label="L" value="V" />)
    const valueEl = container.querySelector('.text-neon-blue')
    expect(valueEl).not.toBeNull()
  })

  it.each([
    ['green', 'text-neon-green'],
    ['pink',  'text-neon-pink'],
    ['gold',  'text-neon-gold'],
  ] as const)('applies correct text class for color=%s', (color, cls) => {
    const { container } = render(<StatCard label="L" value="V" color={color} />)
    expect(container.querySelector(`.${cls}`)).not.toBeNull()
  })
})
