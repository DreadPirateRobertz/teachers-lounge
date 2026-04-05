'use client'

import { useEffect, useRef, useState } from 'react'
import type { UploadedMaterial } from '@/components/materials/MaterialUpload'

/** How often (ms) to poll the status endpoint for non-terminal materials. */
export const POLL_INTERVAL_MS = 3_000

/** Statuses that indicate polling should stop. */
const TERMINAL: ReadonlySet<UploadedMaterial['status']> = new Set(['complete', 'failed'])

/**
 * Poll the material status endpoint until a terminal state is reached.
 *
 * Starts a 3-second polling interval when `materialId` is provided and
 * `initialStatus` is non-terminal (`'pending'` or `'processing'`).  The
 * interval is cleared as soon as the status becomes `'complete'` or
 * `'failed'`, or when the component unmounts.
 *
 * Network errors during polling are swallowed — the status simply does not
 * update on that tick and polling retries on the next interval.
 *
 * @param materialId    - UUID of the material to poll, or `null` to disable.
 * @param initialStatus - Status value received immediately after upload.
 * @returns The current (possibly updated) processing status.
 */
export function useMaterialStatus(
  materialId: string | null,
  initialStatus: UploadedMaterial['status'],
): UploadedMaterial['status'] {
  const [status, setStatus] = useState<UploadedMaterial['status']>(initialStatus)

  // Keep a ref so the interval callback always reads the latest status without
  // re-creating the interval on every status change.
  const statusRef = useRef<UploadedMaterial['status']>(status)
  statusRef.current = status

  useEffect(() => {
    if (!materialId || TERMINAL.has(initialStatus)) return

    const id = setInterval(async () => {
      // Stop early if we already reached a terminal state between ticks.
      if (TERMINAL.has(statusRef.current)) {
        clearInterval(id)
        return
      }

      try {
        const res = await fetch(`/api/materials/${materialId}/status`)
        if (!res.ok) return
        const data = (await res.json()) as { status: UploadedMaterial['status'] }
        const next = data.status
        setStatus(next)
        statusRef.current = next
        if (TERMINAL.has(next)) clearInterval(id)
      } catch {
        // Non-fatal — polling will retry on the next tick.
      }
    }, POLL_INTERVAL_MS)

    return () => clearInterval(id)
  }, [materialId, initialStatus])

  return status
}
