import { NextRequest, NextResponse } from 'next/server'

const GAMING_SERVICE_URL = process.env.GAMING_SERVICE_URL

function authHeader(req: NextRequest): Record<string, string> {
  const bearer =
    req.headers.get('authorization') ||
    (req.cookies.get('tl_token')?.value
      ? `Bearer ${req.cookies.get('tl_token')!.value}`
      : null)
  return bearer ? { Authorization: bearer } : {}
}

// GET /api/gaming/quests — fetch daily quest states for the authenticated user
export async function GET(req: NextRequest) {
  if (!GAMING_SERVICE_URL) {
    return NextResponse.json(mockDailyQuests())
  }

  const upstream = await fetch(`${GAMING_SERVICE_URL}/gaming/quests/daily`, {
    headers: { 'Content-Type': 'application/json', ...authHeader(req) },
    cache: 'no-store',
  })

  if (!upstream.ok) {
    return NextResponse.json(
      { error: 'Failed to fetch quests from gaming service' },
      { status: upstream.status },
    )
  }

  const data = await upstream.json()
  return NextResponse.json(data)
}

// POST /api/gaming/quests — advance quest progress by action
export async function POST(req: NextRequest) {
  if (!GAMING_SERVICE_URL) {
    return NextResponse.json(mockProgressResponse())
  }

  const body = await req.json()

  const upstream = await fetch(`${GAMING_SERVICE_URL}/gaming/quests/progress`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeader(req) },
    body: JSON.stringify(body),
  })

  if (!upstream.ok) {
    return NextResponse.json(
      { error: 'Failed to update quest progress' },
      { status: upstream.status },
    )
  }

  const data = await upstream.json()
  return NextResponse.json(data)
}

// ── Mock data (used when GAMING_SERVICE_URL is not set) ──────────────────────

function mockDailyQuests() {
  return {
    quests: [
      {
        id: 'questions_answered',
        title: 'Question Seeker',
        description: 'Answer 5 questions today',
        progress: 3,
        target: 5,
        completed: false,
        xp_reward: 25,
        gems_reward: 5,
      },
      {
        id: 'keep_streak_alive',
        title: 'Streak Keeper',
        description: 'Keep your learning streak alive',
        progress: 1,
        target: 1,
        completed: true,
        xp_reward: 35,
        gems_reward: 10,
      },
      {
        id: 'master_new_concept',
        title: 'Concept Pioneer',
        description: 'Master a new concept',
        progress: 0,
        target: 1,
        completed: false,
        xp_reward: 75,
        gems_reward: 20,
      },
    ],
  }
}

function mockProgressResponse() {
  return {
    ...mockDailyQuests(),
    xp_awarded: 0,
    gems_awarded: 0,
  }
}
