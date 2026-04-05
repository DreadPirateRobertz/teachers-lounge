import { NextRequest, NextResponse } from 'next/server'

const TUTORING_SERVICE_URL = process.env.TUTORING_SERVICE_URL

/** Maximum number of messages accepted per request (prevents context-window stuffing). */
const MAX_MESSAGES = 50
/** Maximum character length for a single message content string. */
const MAX_MESSAGE_LENGTH = 4000

// Mock streaming response for Phase 1 (before Tutoring Service is live)
async function* mockStream(userMessage: string): AsyncGenerator<string> {
  const responses = [
    `Hey there! I'm **Professor Nova** — your personal AI tutor. `,
    `You asked: *"${userMessage}"*\n\n`,
    `Great question! Let me break this down for you.\n\n`,
    `In Phase 1 I'm running without my full knowledge base, but once `,
    `the Tutoring Service is connected I'll be able to answer from your `,
    `course materials with full RAG-powered context.\n\n`,
    `For now, I'm here to help you explore the TeachersLounge interface. `,
    `Try asking me anything — and watch the XP bar at the bottom! ⚡`,
  ]
  for (const chunk of responses) {
    yield chunk
    await new Promise((r) => setTimeout(r, 40))
  }
}

export async function POST(req: NextRequest) {
  const body = await req.json()
  const rawMessages: unknown[] = Array.isArray(body?.messages) ? body.messages : []

  // Validate message count — prevents context-window stuffing attacks.
  if (rawMessages.length > MAX_MESSAGES) {
    return NextResponse.json({ error: 'too many messages' }, { status: 400 })
  }

  // Validate and truncate individual message content.
  const messages = rawMessages.map((m) => {
    const msg = m as Record<string, unknown>
    const content = typeof msg.content === 'string' ? msg.content.slice(0, MAX_MESSAGE_LENGTH) : ''
    return { ...msg, content }
  })

  const lastMessage = messages[messages.length - 1]?.content ?? ''

  // Forward to Tutoring Service when available
  if (TUTORING_SERVICE_URL) {
    const authHeader =
      req.headers.get('authorization') ||
      (req.cookies.get('tl_token')?.value
        ? `Bearer ${req.cookies.get('tl_token')!.value}`
        : undefined)
    const upstream = await fetch(`${TUTORING_SERVICE_URL}/v1/chat`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(authHeader ? { Authorization: authHeader } : {}),
      },
      body: JSON.stringify({ messages }),
    })
    if (upstream.ok && upstream.body) {
      return new Response(upstream.body, {
        headers: {
          'Content-Type': 'text/plain; charset=utf-8',
          'X-Content-Type-Options': 'nosniff',
        },
      })
    }
  }

  // Fall through to mock
  const encoder = new TextEncoder()
  const stream = new ReadableStream({
    async start(controller) {
      for await (const chunk of mockStream(lastMessage)) {
        controller.enqueue(encoder.encode(chunk))
      }
      controller.close()
    },
  })

  return new Response(stream, {
    headers: {
      'Content-Type': 'text/plain; charset=utf-8',
      'X-Content-Type-Options': 'nosniff',
    },
  })
}
