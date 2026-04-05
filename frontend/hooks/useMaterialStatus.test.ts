/**
 * @jest-environment jsdom
 *
 * Tests for useMaterialStatus hook.
 *
 * Covers: initial state return, no polling for terminal statuses, polling
 * triggers status updates, polling stops on terminal status, and network
 * errors are swallowed.
 */
import { renderHook, act, waitFor } from '@testing-library/react'
import { useMaterialStatus, POLL_INTERVAL_MS } from './useMaterialStatus'

const MATERIAL_ID = 'a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5'

const mockFetch = jest.fn()

beforeEach(() => {
  jest.useFakeTimers()
  global.fetch = mockFetch
})

afterEach(() => {
  jest.useRealTimers()
  mockFetch.mockReset()
})

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Build a resolved fetch mock that returns `{ status }`.
 *
 * @param status - The `status` field to include in the JSON response.
 * @returns A promise-resolving mock compatible with global.fetch.
 */
function mockStatusResponse(status: string) {
  return Promise.resolve({
    ok: true,
    json: async () => ({ status }),
  })
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useMaterialStatus — initial state', () => {
  it('returns initialStatus immediately without fetching', () => {
    const { result } = renderHook(() =>
      useMaterialStatus(MATERIAL_ID, 'pending'),
    )
    expect(result.current).toBe('pending')
    expect(mockFetch).not.toHaveBeenCalled()
  })

  it('returns initialStatus when materialId is null', () => {
    const { result } = renderHook(() => useMaterialStatus(null, 'pending'))
    expect(result.current).toBe('pending')
  })
})

describe('useMaterialStatus — no polling for terminal statuses', () => {
  it('does not start polling when initialStatus is "complete"', () => {
    renderHook(() => useMaterialStatus(MATERIAL_ID, 'complete'))
    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS * 3) })
    expect(mockFetch).not.toHaveBeenCalled()
  })

  it('does not start polling when initialStatus is "failed"', () => {
    renderHook(() => useMaterialStatus(MATERIAL_ID, 'failed'))
    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS * 3) })
    expect(mockFetch).not.toHaveBeenCalled()
  })
})

describe('useMaterialStatus — polling', () => {
  it('polls after one interval and updates status', async () => {
    mockFetch.mockReturnValueOnce(mockStatusResponse('processing'))

    const { result } = renderHook(() => useMaterialStatus(MATERIAL_ID, 'pending'))

    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS) })

    await waitFor(() => expect(result.current).toBe('processing'))
    expect(mockFetch).toHaveBeenCalledTimes(1)
    expect(mockFetch).toHaveBeenCalledWith(
      `/api/materials/${MATERIAL_ID}/status`,
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it('sends credentials: include so the auth cookie is forwarded', async () => {
    mockFetch.mockReturnValueOnce(mockStatusResponse('complete'))

    renderHook(() => useMaterialStatus(MATERIAL_ID, 'pending'))
    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS) })

    await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(1))
    expect(mockFetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it('stops polling when status becomes "complete"', async () => {
    mockFetch
      .mockReturnValueOnce(mockStatusResponse('processing'))
      .mockReturnValueOnce(mockStatusResponse('complete'))

    const { result } = renderHook(() => useMaterialStatus(MATERIAL_ID, 'pending'))

    // First tick → processing
    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS) })
    await waitFor(() => expect(result.current).toBe('processing'))

    // Second tick → complete
    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS) })
    await waitFor(() => expect(result.current).toBe('complete'))

    // Third tick — interval should be cleared; fetch must NOT be called again
    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS) })
    expect(mockFetch).toHaveBeenCalledTimes(2)
  })

  it('stops polling when status becomes "failed"', async () => {
    mockFetch.mockReturnValueOnce(mockStatusResponse('failed'))

    const { result } = renderHook(() => useMaterialStatus(MATERIAL_ID, 'pending'))

    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS) })
    await waitFor(() => expect(result.current).toBe('failed'))

    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS) })
    expect(mockFetch).toHaveBeenCalledTimes(1)
  })

  it('clears the interval on unmount', () => {
    mockFetch.mockReturnValue(mockStatusResponse('processing'))

    const { unmount } = renderHook(() => useMaterialStatus(MATERIAL_ID, 'pending'))
    unmount()

    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS * 5) })
    expect(mockFetch).not.toHaveBeenCalled()
  })
})

describe('useMaterialStatus — error resilience', () => {
  it('swallows network errors and retries on the next tick', async () => {
    mockFetch
      .mockReturnValueOnce(Promise.reject(new Error('ECONNREFUSED')))
      .mockReturnValueOnce(mockStatusResponse('complete'))

    const { result } = renderHook(() => useMaterialStatus(MATERIAL_ID, 'pending'))

    // First tick — network error, status stays pending
    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS) })
    await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(1))
    expect(result.current).toBe('pending')

    // Second tick — success
    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS) })
    await waitFor(() => expect(result.current).toBe('complete'))
  })

  it('swallows non-ok responses without updating status', async () => {
    mockFetch.mockReturnValueOnce(
      Promise.resolve({ ok: false, json: async () => ({ detail: 'error' }) }),
    )

    const { result } = renderHook(() => useMaterialStatus(MATERIAL_ID, 'pending'))

    act(() => { jest.advanceTimersByTime(POLL_INTERVAL_MS) })
    await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(1))
    expect(result.current).toBe('pending')
  })
})
