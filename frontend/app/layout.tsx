import type { Metadata } from 'next'
import { headers } from 'next/headers'
import { GeistSans } from 'geist/font/sans'
import { GeistMono } from 'geist/font/mono'
import './globals.css'
import 'katex/dist/katex.min.css'

export const metadata: Metadata = {
  title: 'TeachersLounge — AI Tutor',
  description: 'AI-powered personalized tutor with gamification',
}

/**
 * Root layout for the TeachersLounge App Router tree.
 *
 * Reads the per-request CSP nonce injected by `middleware.ts` via the
 * `x-nonce` request header.  The nonce must be forwarded to any
 * `<Script nonce={nonce} />` elements so Next.js hydration inline scripts
 * are permitted under the `'nonce-<value>'` CSP directive without
 * requiring `unsafe-inline`.
 */
export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const headersList = await headers()
  // nonce is set by middleware.ts (tl-ixk) — present on all page requests.
  const _nonce = headersList.get('x-nonce') ?? ''

  return (
    <html lang="en" className={`${GeistSans.variable} ${GeistMono.variable}`}>
      <body className="font-sans bg-bg-deep text-text-base antialiased">{children}</body>
    </html>
  )
}
