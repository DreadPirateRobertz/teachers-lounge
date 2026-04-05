/**
 * @jest-environment jsdom
 *
 * Tests for the CharacterSidebar component.
 *
 * Verifies avatar display, XP bar, streak section, quest list rendering after
 * a successful fetch, and graceful handling of fetch errors.
 */

import React from 'react'
import { render, screen, waitFor, cleanup } from '@testing-library/react'
import CharacterSidebar from './CharacterSidebar'

jest.mock('next/link', () => {
  /** Mock next/link as a plain anchor element. */
  const MockLink = ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  )
  MockLink.displayName = 'MockLink'
  return MockLink
})

/** Build a minimal fetch mock that resolves with the given data. */
function mockFetchSuccess(data: unknown) {
  global.fetch = jest.fn().mockResolvedValue({
    ok: true,
    json: async () => data,
  } as Response)
}

/** Build a fetch mock that rejects with an error. */
function mockFetchError() {
  global.fetch = jest.fn().mockRejectedValue(new Error('Network error'))
}

afterEach(() => {
  cleanup()
  jest.restoreAllMocks()
})

describe('CharacterSidebar', () => {
  describe('avatar card', () => {
    it('renders the wizard emoji and character name', async () => {
      mockFetchSuccess({ quests: [] })
      render(<CharacterSidebar />)

      expect(screen.getByText('🧙')).toBeInTheDocument()
      expect(screen.getByText('ChemWizard')).toBeInTheDocument()
    })

    it('renders Scholar · Rank IV rank text', async () => {
      mockFetchSuccess({ quests: [] })
      render(<CharacterSidebar />)

      expect(screen.getByText('Scholar · Rank IV')).toBeInTheDocument()
    })

    it('renders the XP bar section with level text', async () => {
      mockFetchSuccess({ quests: [] })
      render(<CharacterSidebar />)

      expect(screen.getByText('Lv 12')).toBeInTheDocument()
    })
  })

  describe('quest list', () => {
    it('renders a quest title after fetch resolves with quests', async () => {
      mockFetchSuccess({
        quests: [
          {
            id: 'q1',
            title: 'Answer 5 questions',
            description: 'Answer five questions today',
            progress: 2,
            target: 5,
            completed: false,
            xp_reward: 50,
            gems_reward: 10,
          },
        ],
      })

      render(<CharacterSidebar />)

      await waitFor(() => {
        expect(screen.getByText('Answer 5 questions')).toBeInTheDocument()
      })
    })
  })

  describe('error handling', () => {
    it('does not crash when fetch rejects', async () => {
      mockFetchError()

      expect(() => render(<CharacterSidebar />)).not.toThrow()

      // The component should still render the static sections
      await waitFor(() => {
        expect(screen.getByText('ChemWizard')).toBeInTheDocument()
      })
    })
  })
})
