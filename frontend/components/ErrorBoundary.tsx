'use client'

import React, { Component, type ReactNode, type ErrorInfo } from 'react'

interface Props {
  /** Content to protect from blank-page crashes. */
  children: ReactNode
  /** Optional custom fallback UI. Overrides the default error card. */
  fallback?: ReactNode
  /** Human-readable name shown in the default fallback and console log. */
  componentName?: string
}

interface State {
  hasError: boolean
  error: Error | null
}

/**
 * React error boundary that catches render errors in its child subtree
 * and displays a fallback UI instead of a blank page.
 *
 * Usage:
 *   <ErrorBoundary componentName="Leaderboard">
 *     <LeaderboardPanel />
 *   </ErrorBoundary>
 */
export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    const name = this.props.componentName ?? 'Component'
    console.error(`[ErrorBoundary] ${name} crashed:`, error, info.componentStack)
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null })
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback !== undefined) {
        return this.props.fallback
      }

      const name = this.props.componentName ?? 'This section'
      return (
        <div className="flex flex-col items-center justify-center gap-3 p-4 bg-bg-card border border-red-500/30 rounded-lg text-center">
          <span className="text-2xl">⚠️</span>
          <p className="text-sm text-text-dim">{name} failed to load.</p>
          <button
            onClick={this.handleReset}
            className="text-xs text-neon-blue border border-neon-blue/30 px-3 py-1.5 rounded hover:bg-neon-blue/10 transition-colors"
          >
            Try again
          </button>
        </div>
      )
    }

    return this.props.children
  }
}
