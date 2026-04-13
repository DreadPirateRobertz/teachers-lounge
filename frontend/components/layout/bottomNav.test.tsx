/**
 * @jest-environment jsdom
 *
 * Tests for BottomNav — mobile tab bar (hidden on md+ screens).
 * Verifies tab rendering, active-state highlighting, and navigation links.
 */
import React from 'react'
import { render, screen } from '@testing-library/react'
import BottomNav from './BottomNav'

jest.mock('next/link', () => {
  const MockLink = ({
    href,
    children,
    ...rest
  }: {
    href: string
    children: React.ReactNode
    [key: string]: unknown
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  )
  MockLink.displayName = 'MockLink'
  return MockLink
})

jest.mock('next/navigation', () => ({
  usePathname: jest.fn(),
}))

import { usePathname } from 'next/navigation'
const mockUsePathname = usePathname as jest.MockedFunction<typeof usePathname>

describe('BottomNav', () => {
  beforeEach(() => {
    mockUsePathname.mockReturnValue('/')
  })

  it('renders all four tabs', () => {
    render(<BottomNav />)
    expect(screen.getByRole('link', { name: /chat/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /battle/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /shop/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /profile/i })).toBeInTheDocument()
  })

  it('marks the active tab when on the home route', () => {
    mockUsePathname.mockReturnValue('/')
    render(<BottomNav />)
    const chatLink = screen.getByRole('link', { name: /chat/i })
    expect(chatLink).toHaveAttribute('aria-current', 'page')
  })

  it('marks battle tab active on /boss route', () => {
    mockUsePathname.mockReturnValue('/boss/dragon')
    render(<BottomNav />)
    const battleLink = screen.getByRole('link', { name: /battle/i })
    expect(battleLink).toHaveAttribute('aria-current', 'page')
  })

  it('marks shop tab active on /shop route', () => {
    mockUsePathname.mockReturnValue('/shop')
    render(<BottomNav />)
    const shopLink = screen.getByRole('link', { name: /shop/i })
    expect(shopLink).toHaveAttribute('aria-current', 'page')
  })

  it('marks profile tab active on /profile route', () => {
    mockUsePathname.mockReturnValue('/profile')
    render(<BottomNav />)
    const profileLink = screen.getByRole('link', { name: /profile/i })
    expect(profileLink).toHaveAttribute('aria-current', 'page')
  })

  it('renders inside a nav landmark', () => {
    render(<BottomNav />)
    expect(screen.getByRole('navigation', { name: /mobile tabs/i })).toBeInTheDocument()
  })
})
