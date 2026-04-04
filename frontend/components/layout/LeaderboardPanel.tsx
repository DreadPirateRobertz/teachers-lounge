'use client'

import { useState, useEffect, useCallback } from 'react'

type Period = 'all_time' | 'weekly' | 'monthly'

interface LeaderboardEntry {
  user_id: string
  xp: number
  rank: number
  display_name?: string
}

interface LeaderboardData {
  top_10: LeaderboardEntry[]
  user_rank: LeaderboardEntry | null
}

interface FriendLeaderboardData {
  friends: LeaderboardEntry[]
  user_rank: LeaderboardEntry | null
}

// TODO: Replace with real friends list from user service when available
const MOCK_FRIEND_IDS = ['friend-1', 'friend-2', 'friend-3']

function rankDelta(currentRank: number, previousRank: number | undefined): number | null {
  if (previousRank === undefined || previousRank === 0) return null
  return previousRank - currentRank // positive = moved up
}

export default function LeaderboardPanel() {
  const [period, setPeriod] = useState<Period>('all_time')
  const [friendsOnly, setFriendsOnly] = useState(false)
  const [data, setData] = useState<LeaderboardData | null>(null)
  const [friendData, setFriendData] = useState<FriendLeaderboardData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Store previous ranks to compute deltas across period switches
  const [prevRanks, setPrevRanks] = useState<Map<string, number>>(new Map())

  const fetchLeaderboard = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const res = await fetch(`/api/gaming/leaderboard?period=${period}`)
      if (!res.ok) throw new Error('Failed to load leaderboard')
      const json: LeaderboardData = await res.json()

      // Capture current ranks before updating so we can show deltas
      if (data?.top_10) {
        const ranks = new Map<string, number>()
        for (const entry of data.top_10) {
          ranks.set(entry.user_id, entry.rank)
        }
        if (data.user_rank) {
          ranks.set(data.user_rank.user_id, data.user_rank.rank)
        }
        setPrevRanks(ranks)
      }

      setData(json)
    } catch {
      setError('Could not load leaderboard')
    } finally {
      setLoading(false)
    }
  }, [period]) // eslint-disable-line react-hooks/exhaustive-deps

  const fetchFriends = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const res = await fetch(
        `/api/gaming/leaderboard/friends?friends=${MOCK_FRIEND_IDS.join(',')}`,
      )
      if (!res.ok) throw new Error('Failed to load friend leaderboard')
      const json: FriendLeaderboardData = await res.json()
      setFriendData(json)
    } catch {
      setError('Could not load friend rankings')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (friendsOnly) {
      fetchFriends()
    } else {
      fetchLeaderboard()
    }
  }, [friendsOnly, fetchLeaderboard, fetchFriends])

  const entries = friendsOnly
    ? friendData?.friends ?? []
    : data?.top_10 ?? []
  const userRank = friendsOnly ? friendData?.user_rank : data?.user_rank

  return (
    <div className="flex flex-col gap-3">
      {/* Period tabs */}
      <div className="flex border border-border-dim rounded-lg overflow-hidden">
        {(['all_time', 'weekly', 'monthly'] as Period[]).map((p) => (
          <button
            key={p}
            onClick={() => setPeriod(p)}
            disabled={friendsOnly}
            className={`flex-1 py-1.5 text-[11px] font-medium transition-colors ${
              !friendsOnly && period === p
                ? 'bg-neon-blue/15 text-neon-blue border-b-2 border-neon-blue'
                : 'text-text-dim hover:text-text-base disabled:opacity-40 disabled:cursor-not-allowed'
            }`}
          >
            {p === 'all_time' ? 'All Time' : p === 'weekly' ? 'Weekly' : 'Monthly'}
          </button>
        ))}
      </div>

      {/* Friend filter toggle */}
      <button
        onClick={() => setFriendsOnly((v) => !v)}
        className={`flex items-center gap-1.5 px-2 py-1 rounded text-[11px] font-medium transition-colors self-start ${
          friendsOnly
            ? 'bg-neon-green/15 text-neon-green border border-neon-green/30'
            : 'bg-bg-card text-text-dim border border-border-dim hover:text-text-base hover:border-border-mid'
        }`}
      >
        <span className="text-xs">{friendsOnly ? '👥' : '🌐'}</span>
        {friendsOnly ? 'Friends' : 'Global'}
      </button>

      {/* Loading */}
      {loading && (
        <div className="flex flex-col gap-1">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-8 bg-bg-card border border-border-dim rounded animate-pulse" />
          ))}
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="text-xs text-red-400 bg-red-400/10 border border-red-400/20 rounded px-2 py-1.5">
          {error}
        </div>
      )}

      {/* Leaderboard rows */}
      {!loading && !error && (
        <div className="flex flex-col gap-1">
          {entries.length === 0 && (
            <div className="text-xs text-text-dim text-center py-4">
              No rankings yet
            </div>
          )}
          {entries.map((entry) => {
            const isMe = userRank?.user_id === entry.user_id
            const delta = rankDelta(entry.rank, prevRanks.get(entry.user_id))

            return (
              <div
                key={entry.user_id}
                className={`flex items-center gap-2 px-2 py-1.5 rounded text-xs ${
                  isMe
                    ? 'bg-neon-blue/10 border border-neon-blue/30'
                    : 'bg-bg-card border border-border-dim'
                }`}
              >
                {/* Rank */}
                <span
                  className={`font-mono font-bold w-5 flex-shrink-0 text-right ${
                    entry.rank === 1
                      ? 'text-neon-gold'
                      : entry.rank === 2
                        ? 'text-gray-300'
                        : entry.rank === 3
                          ? 'text-orange-400'
                          : 'text-text-dim'
                  }`}
                >
                  {entry.rank}
                </span>

                {/* Rank delta indicator */}
                <RankDelta delta={delta} />

                {/* Name */}
                <span
                  className={`flex-1 truncate ${
                    isMe ? 'text-neon-blue font-medium' : 'text-text-base'
                  }`}
                >
                  {entry.display_name || entry.user_id.slice(0, 8)}
                  {isMe && ' (you)'}
                </span>

                {/* XP */}
                <span className="font-mono text-[10px] text-text-dim flex-shrink-0">
                  {entry.xp.toLocaleString()} xp
                </span>
              </div>
            )
          })}

          {/* User rank footer (if not in top 10) */}
          {userRank &&
            !entries.some((e) => e.user_id === userRank.user_id) && (
              <div className="mt-1 pt-1 border-t border-border-dim">
                <div className="flex items-center gap-2 px-2 py-1.5 rounded text-xs bg-neon-blue/10 border border-neon-blue/30">
                  <span className="font-mono font-bold w-5 flex-shrink-0 text-right text-text-dim">
                    {userRank.rank}
                  </span>
                  <RankDelta delta={rankDelta(userRank.rank, prevRanks.get(userRank.user_id))} />
                  <span className="flex-1 truncate text-neon-blue font-medium">
                    {userRank.display_name || userRank.user_id.slice(0, 8)} (you)
                  </span>
                  <span className="font-mono text-[10px] text-text-dim flex-shrink-0">
                    {userRank.xp.toLocaleString()} xp
                  </span>
                </div>
              </div>
            )}
        </div>
      )}
    </div>
  )
}

function RankDelta({ delta }: { delta: number | null }) {
  if (delta === null) return <span className="w-3 flex-shrink-0" />

  if (delta > 0) {
    return (
      <span className="w-3 flex-shrink-0 text-[10px] text-neon-green font-mono" title={`Up ${delta}`}>
        +{delta}
      </span>
    )
  }

  if (delta < 0) {
    return (
      <span className="w-3 flex-shrink-0 text-[10px] text-red-400 font-mono" title={`Down ${Math.abs(delta)}`}>
        {delta}
      </span>
    )
  }

  return (
    <span className="w-3 flex-shrink-0 text-[10px] text-text-dim font-mono" title="No change">
      =
    </span>
  )
}
