import { useCallback, useEffect, useState } from 'react'
import { shortcutsBlocked } from '../lib/keyboard'
import { useStoredState } from '../lib/storage'

const IDLE_MS = 3000

/**
 * Focus mode hides the dashboard, leaving the video background, the music player, and a large
 * clock: ambient music and scenery for focusing. It is remembered across reloads. F toggles it
 * and Esc leaves it, except while typing in a field. While focused and the mouse is still,
 * the cursor and the toggle button fade out.
 */
export function useFocusMode() {
  const [focused, setStoredFocused] = useStoredState('focus-mode', false)
  const [idle, setIdle] = useState(false)
  // Entering or leaving focus mode counts as activity, so the 3s idle countdown starts fresh.
  const setFocused = useCallback(
    (value: boolean) => {
      setIdle(false)
      setStoredFocused(value)
    },
    [setStoredFocused],
  )

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      // Also skipped while the music library is open, since it handles Esc itself.
      if (shortcutsBlocked(e)) return
      if (e.key === 'f' || e.key === 'F') setFocused(!focused)
      else if (e.key === 'Escape' && focused) setFocused(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [focused, setFocused])

  // Track mouse activity only while focused; idle resets whenever focus mode turns off.
  useEffect(() => {
    if (!focused) return
    let timer = setTimeout(() => setIdle(true), IDLE_MS)
    const onActivity = () => {
      setIdle(false)
      clearTimeout(timer)
      timer = setTimeout(() => setIdle(true), IDLE_MS)
    }
    window.addEventListener('mousemove', onActivity)
    window.addEventListener('pointerdown', onActivity)
    return () => {
      clearTimeout(timer)
      window.removeEventListener('mousemove', onActivity)
      window.removeEventListener('pointerdown', onActivity)
    }
  }, [focused])

  return { focused, idle: focused && idle, toggle: () => setFocused(!focused) }
}
