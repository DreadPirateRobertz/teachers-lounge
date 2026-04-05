'use client'

export interface AuthResponse {
  access_token: string
  user: {
    id: string
    email: string
    display_name: string
    avatar_emoji: string
    subscription_status: string
    has_completed_onboarding: boolean
  }
}

export async function login(email: string, password: string): Promise<AuthResponse> {
  const res = await fetch('/api/user/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || 'Login failed')
  return data
}

export async function register(
  email: string,
  password: string,
  displayName: string,
): Promise<AuthResponse> {
  const res = await fetch('/api/user/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, display_name: displayName }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || 'Registration failed')
  return data
}

export async function logout(): Promise<void> {
  await fetch('/api/user/auth/logout', { method: 'POST' })
  window.location.href = '/login'
}
