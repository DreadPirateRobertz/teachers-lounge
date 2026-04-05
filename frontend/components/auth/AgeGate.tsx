'use client'

/**
 * AgeGate — guards content that requires parental consent for minor accounts.
 *
 * For `standard` accounts (adults) or minors who have already received
 * guardian consent, the wrapped children are rendered normally.  For minors
 * without consent, a blocking prompt is shown with a link to request consent.
 *
 * Usage::
 *
 *   <AgeGate accountType={user.account_type} guardianConsentAt={user.guardian_consent_at}>
 *     <ProtectedFeature />
 *   </AgeGate>
 */
import Link from 'next/link'

/** Possible account types returned by the user-service. */
export type AccountType = 'standard' | 'minor'

interface AgeGateProps {
  /** The user's account type.  Undefined for unauthenticated guests. */
  accountType: AccountType | undefined
  /**
   * ISO 8601 timestamp of guardian consent, or null/undefined if not yet
   * granted.
   */
  guardianConsentAt: string | null | undefined
  /** Content to render when the user passes the gate. */
  children: React.ReactNode
}

/**
 * Returns true when the given user requires guardian consent to proceed.
 *
 * @param accountType - The user's account classification.
 * @param guardianConsentAt - Timestamp of existing consent, if any.
 * @returns Whether a consent block should be shown.
 */
function needsConsent(
  accountType: AccountType | undefined,
  guardianConsentAt: string | null | undefined,
): boolean {
  return accountType === 'minor' && !guardianConsentAt
}

/**
 * Consent-required blocking screen displayed to unverified minors.
 */
function ConsentRequired() {
  return (
    <div className="flex flex-col items-center justify-center min-h-[300px] gap-6 p-8 text-center">
      <span className="text-5xl" aria-hidden="true">
        🔒
      </span>
      <h2 className="text-lg font-mono font-bold text-text-base">Parental Consent Required</h2>
      <p className="text-sm text-text-dim max-w-xs font-mono">
        This account is registered as a minor. A parent or guardian must approve access before you
        can continue.
      </p>
      <Link
        href="/consent/request"
        className="px-5 py-2.5 rounded-lg font-mono text-sm font-bold
          bg-neon-blue/10 border border-neon-blue/40 text-neon-blue
          hover:bg-neon-blue/20 transition-colors"
        aria-label="Request consent"
      >
        Request Consent
      </Link>
    </div>
  )
}

/**
 * Conditionally renders {@link ConsentRequired} or the wrapped children based
 * on account type and consent status.
 */
export default function AgeGate({ accountType, guardianConsentAt, children }: AgeGateProps) {
  if (needsConsent(accountType, guardianConsentAt)) {
    return <ConsentRequired />
  }
  return <>{children}</>
}
