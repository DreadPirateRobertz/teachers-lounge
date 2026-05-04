import { NextRequest, NextResponse } from 'next/server'

/** Tutoring service base URL — when set, requests are proxied upstream. */
const TUTORING_SERVICE_URL = process.env.TUTORING_SERVICE_URL

/** Hard cap on the text payload to keep TTS calls cheap. */
const MAX_TTS_CHARS = 1500

/** Allowed voice profiles. Anything else falls back to "nova". */
const ALLOWED_VOICES = new Set(['nova', 'sage', 'amber'])

/**
 * POST /api/tts/speak
 *
 * Request body: `{ text: string, voice?: 'nova' | 'sage' | 'amber' }`.
 * Forwards to the tutoring-service TTS endpoint when `TUTORING_SERVICE_URL`
 * is configured; otherwise returns 503 so the frontend can degrade gracefully.
 *
 * Audio is streamed back unchanged so the browser can pipe it into
 * `HTMLAudioElement` or the Web Audio API without intermediate buffering.
 */
export async function POST(req: NextRequest) {
  let body: unknown
  try {
    body = await req.json()
  } catch {
    return NextResponse.json({ error: 'invalid json' }, { status: 400 })
  }

  const raw = body as Record<string, unknown> | null
  const text = typeof raw?.text === 'string' ? raw.text.slice(0, MAX_TTS_CHARS).trim() : ''
  if (!text) {
    return NextResponse.json({ error: 'text required' }, { status: 400 })
  }
  const voice =
    typeof raw?.voice === 'string' && ALLOWED_VOICES.has(raw.voice) ? (raw.voice as string) : 'nova'

  if (!TUTORING_SERVICE_URL) {
    return NextResponse.json({ error: 'tts service not configured' }, { status: 503 })
  }

  const authHeader =
    req.headers.get('authorization') ||
    (req.cookies.get('tl_token')?.value
      ? `Bearer ${req.cookies.get('tl_token')!.value}`
      : undefined)

  const upstream = await fetch(`${TUTORING_SERVICE_URL}/v1/tts/speak`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(authHeader ? { Authorization: authHeader } : {}),
    },
    body: JSON.stringify({ text, voice }),
  })

  if (!upstream.ok || !upstream.body) {
    return NextResponse.json(
      { error: 'upstream tts failed' },
      { status: upstream.status === 200 ? 502 : upstream.status },
    )
  }

  return new Response(upstream.body, {
    headers: {
      'Content-Type': upstream.headers.get('Content-Type') ?? 'audio/mpeg',
      'X-Content-Type-Options': 'nosniff',
      'Cache-Control': 'no-store',
    },
  })
}
