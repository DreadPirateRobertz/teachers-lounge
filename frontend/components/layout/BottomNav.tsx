'use client'

/**
 * BottomNav — mobile-only tab bar rendered at the bottom of the screen.
 *
 * Hidden on medium-and-up screens (`md:hidden`) where the full three-panel
 * desktop layout is shown instead.  Four tabs map to the primary K-12
 * navigation destinations.
 */
import Link from 'next/link'
import { usePathname } from 'next/navigation'

interface Tab {
  /** URL path this tab links to. */
  href: string
  /** Display label shown below the icon. */
  label: string
  /** Emoji icon for the tab. */
  icon: string
  /** Pathname prefix used to determine the active state. */
  match: string
}

const TABS: Tab[] = [
  { href: '/', label: 'Chat', icon: '💬', match: '/' },
  { href: '/boss', label: 'Battle', icon: '⚔️', match: '/boss' },
  { href: '/shop', label: 'Shop', icon: '💎', match: '/shop' },
  { href: '/profile', label: 'Profile', icon: '👤', match: '/profile' },
]

/**
 * Returns true when the given tab should be highlighted as active.
 *
 * The root tab (`/`) is only active on an exact match; all other tabs are
 * active when the current pathname starts with their match prefix.
 *
 * @param tab - The tab to test.
 * @param pathname - The current URL pathname.
 * @returns Whether the tab is active.
 */
function isActive(tab: Tab, pathname: string | null): boolean {
  if (!pathname) return false
  if (tab.match === '/') return pathname === '/'
  return pathname.startsWith(tab.match)
}

/**
 * Mobile bottom navigation bar with four primary tabs.
 *
 * Renders a `<nav>` fixed to the bottom of the viewport.  The component is
 * wrapped in `md:hidden` so it is only visible on small screens.
 */
export default function BottomNav() {
  const pathname = usePathname()

  return (
    <nav
      aria-label="Mobile tabs"
      className="md:hidden fixed bottom-0 left-0 right-0 z-50
        flex items-stretch bg-bg-panel border-t border-border-dim
        safe-area-inset-bottom"
    >
      {TABS.map((tab) => {
        const active = isActive(tab, pathname)
        return (
          <Link
            key={tab.href}
            href={tab.href}
            aria-current={active ? 'page' : undefined}
            aria-label={tab.label}
            className={[
              'flex flex-col items-center justify-center flex-1 gap-0.5',
              'py-2 min-h-[56px] text-xs font-mono transition-colors',
              'active:bg-bg-card',
              active ? 'text-neon-blue' : 'text-text-dim hover:text-text-base',
            ].join(' ')}
          >
            <span className="text-lg leading-none" aria-hidden="true">
              {tab.icon}
            </span>
            <span>{tab.label}</span>
          </Link>
        )
      })}
    </nav>
  )
}
