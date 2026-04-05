/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import XpProgressBar from './XpProgressBar'

describe('XpProgressBar', () => {
  it('renders current level', () => {
    render(<XpProgressBar current={500} levelMax={1000} level={5} />)
    expect(screen.getByText('5')).toBeInTheDocument()
  })

  it('renders next level label', () => {
    render(<XpProgressBar current={500} levelMax={1000} level={5} />)
    expect(screen.getByText('→ Lv 6')).toBeInTheDocument()
  })

  it('renders current/max XP values', () => {
    render(<XpProgressBar current={500} levelMax={1000} level={5} />)
    expect(screen.getByText('500 / 1,000')).toBeInTheDocument()
  })

  it('shows 50% for current=500 levelMax=1000', () => {
    render(<XpProgressBar current={500} levelMax={1000} level={5} />)
    expect(screen.getByText('50%')).toBeInTheDocument()
  })

  it('shows 100% when current equals levelMax', () => {
    render(<XpProgressBar current={1000} levelMax={1000} level={5} />)
    expect(screen.getByText('100%')).toBeInTheDocument()
  })

  it('clamps to 100% when current exceeds levelMax', () => {
    render(<XpProgressBar current={1500} levelMax={1000} level={5} />)
    expect(screen.getByText('100%')).toBeInTheDocument()
  })

  it('shows 0% when current is 0', () => {
    render(<XpProgressBar current={0} levelMax={1000} level={1} />)
    expect(screen.getByText('0%')).toBeInTheDocument()
  })

  it('sets fill bar width style to match percentage', () => {
    const { container } = render(<XpProgressBar current={750} levelMax={1000} level={3} />)
    const fill = container.querySelector('[style*="width: 75%"]')
    expect(fill).not.toBeNull()
  })
})
