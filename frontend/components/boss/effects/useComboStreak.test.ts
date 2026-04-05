/**
 * @jest-environment jsdom
 *
 * Tests for useComboStreak hook.
 *
 * Verifies streak level thresholds, labels, glow colors, and isActive flag.
 */

import { renderHook } from '@testing-library/react'
import { useComboStreak } from './useComboStreak'

describe('useComboStreak', () => {
  it('streakLevel is 0 when comboCount < 3', () => {
    const { result } = renderHook(() => useComboStreak(2))
    expect(result.current.streakLevel).toBe(0)
  })

  it('streakLevel is 0 when comboCount is 0', () => {
    const { result } = renderHook(() => useComboStreak(0))
    expect(result.current.streakLevel).toBe(0)
  })

  it('streakLevel is 1 when comboCount is exactly 3', () => {
    const { result } = renderHook(() => useComboStreak(3))
    expect(result.current.streakLevel).toBe(1)
  })

  it('streakLevel is 1 when comboCount is 4', () => {
    const { result } = renderHook(() => useComboStreak(4))
    expect(result.current.streakLevel).toBe(1)
  })

  it('streakLevel is 2 when comboCount >= 5', () => {
    const { result } = renderHook(() => useComboStreak(5))
    expect(result.current.streakLevel).toBe(2)
  })

  it('streakLevel is 2 when comboCount is well above 5', () => {
    const { result } = renderHook(() => useComboStreak(20))
    expect(result.current.streakLevel).toBe(2)
  })

  it('streakLabel is empty string at level 0', () => {
    const { result } = renderHook(() => useComboStreak(0))
    expect(result.current.streakLabel).toBe('')
  })

  it('streakLabel is "1.5× COMBO" at level 1', () => {
    const { result } = renderHook(() => useComboStreak(3))
    expect(result.current.streakLabel).toBe('1.5× COMBO')
  })

  it('streakLabel is "2× COMBO" at level 2', () => {
    const { result } = renderHook(() => useComboStreak(5))
    expect(result.current.streakLabel).toBe('2× COMBO')
  })

  it('glowColor is empty string at level 0', () => {
    const { result } = renderHook(() => useComboStreak(0))
    expect(result.current.glowColor).toBe('')
  })

  it('glowColor is neon-blue at level 1', () => {
    const { result } = renderHook(() => useComboStreak(3))
    expect(result.current.glowColor).toBe('#00aaff')
  })

  it('glowColor is neon-gold at level 2', () => {
    const { result } = renderHook(() => useComboStreak(5))
    expect(result.current.glowColor).toBe('#ffdc00')
  })

  it('isActive is false when comboCount < 3', () => {
    const { result } = renderHook(() => useComboStreak(2))
    expect(result.current.isActive).toBe(false)
  })

  it('isActive is true when comboCount >= 3', () => {
    const { result } = renderHook(() => useComboStreak(3))
    expect(result.current.isActive).toBe(true)
  })

  it('isActive is true at level 2', () => {
    const { result } = renderHook(() => useComboStreak(5))
    expect(result.current.isActive).toBe(true)
  })
})
