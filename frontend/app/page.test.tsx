/**
 * @jest-environment jsdom
 *
 * Tests for the root HomePage component (`app/page.tsx`).
 *
 * All imported layout and chat sub-components are stubbed so tests remain
 * focused on the page shell itself rather than sub-component internals.
 */

import React from 'react'
import { render, screen, cleanup } from '@testing-library/react'
import HomePage from './page'

// ── Module mocks ──────────────────────────────────────────────────────────────

jest.mock('next/link', () => {
  /** Mock next/link as a plain anchor element. */
  const MockLink = ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  )
  MockLink.displayName = 'MockLink'
  return MockLink
})

jest.mock('@/components/layout/AppHeader', () => {
  /** Stub AppHeader for page-level tests. */
  const AppHeader = () => <header data-testid="app-header">AppHeader</header>
  AppHeader.displayName = 'AppHeader'
  return AppHeader
})

jest.mock('@/components/layout/CharacterSidebar', () => {
  /** Stub CharacterSidebar for page-level tests. */
  const CharacterSidebar = () => <aside data-testid="character-sidebar">CharacterSidebar</aside>
  CharacterSidebar.displayName = 'CharacterSidebar'
  return CharacterSidebar
})

jest.mock('@/components/layout/MaterialsSidebar', () => {
  /** Stub MaterialsSidebar for page-level tests. */
  const MaterialsSidebar = () => <aside data-testid="materials-sidebar">MaterialsSidebar</aside>
  MaterialsSidebar.displayName = 'MaterialsSidebar'
  return MaterialsSidebar
})

jest.mock('@/components/layout/XpProgressBar', () => {
  /** Stub XpProgressBar for page-level tests. */
  const XpProgressBar = () => <div data-testid="xp-progress-bar">XpProgressBar</div>
  XpProgressBar.displayName = 'XpProgressBar'
  return XpProgressBar
})

jest.mock('@/components/layout/LevelUpBanner', () => {
  /** Stub LevelUpBanner for page-level tests. */
  const LevelUpBanner = () => <div data-testid="level-up-banner">LevelUpBanner</div>
  LevelUpBanner.displayName = 'LevelUpBanner'
  return LevelUpBanner
})

jest.mock('@/components/chat/ChatPanel', () => {
  /** Stub ChatPanel for page-level tests. */
  const ChatPanel = () => <div data-testid="chat-panel">ChatPanel</div>
  ChatPanel.displayName = 'ChatPanel'
  return ChatPanel
})

// ── Tests ─────────────────────────────────────────────────────────────────────

afterEach(() => {
  cleanup()
})

describe('HomePage', () => {
  it('renders without crashing', () => {
    expect(() => render(<HomePage />)).not.toThrow()
  })

  it('renders the AppHeader', () => {
    render(<HomePage />)
    expect(screen.getByTestId('app-header')).toBeInTheDocument()
  })

  it('renders the CharacterSidebar', () => {
    render(<HomePage />)
    expect(screen.getByTestId('character-sidebar')).toBeInTheDocument()
  })

  it('renders the ChatPanel in the main content area', () => {
    render(<HomePage />)
    expect(screen.getByTestId('chat-panel')).toBeInTheDocument()
  })

  it('renders the MaterialsSidebar', () => {
    render(<HomePage />)
    expect(screen.getByTestId('materials-sidebar')).toBeInTheDocument()
  })

  it('renders the XpProgressBar', () => {
    render(<HomePage />)
    expect(screen.getByTestId('xp-progress-bar')).toBeInTheDocument()
  })

  it('renders the LevelUpBanner', () => {
    render(<HomePage />)
    expect(screen.getByTestId('level-up-banner')).toBeInTheDocument()
  })
})
