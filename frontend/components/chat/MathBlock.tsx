'use client'

// KaTeX math rendering scaffold — Phase 1
//
// KaTeX is installed (katex package) and ready for wiring.
// Usage:
//   Inline math:  $x^2 + y^2 = r^2$
//   Block math:   $$\int_0^\infty e^{-x^2} dx = \frac{\sqrt{\pi}}{2}$$
//
// The ChatMessage component detects these delimiters and renders them via
// MathBlock. Full KaTeX render is enabled in Phase 2 when the Tutoring
// Service begins generating LaTeX expressions in responses.

import { useEffect, useRef } from 'react'

interface Props {
  expression: string
  block?: boolean
}

export default function MathBlock({ expression, block = false }: Props) {
  const ref = useRef<HTMLSpanElement & HTMLDivElement>(null)

  useEffect(() => {
    if (!ref.current) return

    // Dynamically import katex to avoid SSR issues
    import('katex').then(({ default: katex }) => {
      if (!ref.current) return
      katex.render(expression, ref.current, {
        throwOnError: false,
        displayMode: block,
        trust: false,
        strict: 'warn',
      })
    })
  }, [expression, block])

  if (block) {
    return (
      <div
        ref={ref as React.RefObject<HTMLDivElement>}
        className="my-3 overflow-x-auto text-center py-2 px-4 bg-bg-card border border-border-dim rounded-lg text-text-base"
        aria-label={`Math: ${expression}`}
      />
    )
  }

  return (
    <span
      ref={ref as React.RefObject<HTMLSpanElement>}
      className="inline-block align-middle text-text-base"
      aria-label={`Math: ${expression}`}
    />
  )
}
