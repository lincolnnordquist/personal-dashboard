import { useState } from 'react'

// useStoredState is useState persisted to localStorage. Storage can be unavailable (private
// windows, blocked site data), so failures fall back to the default and are otherwise ignored.
export function useStoredState<T>(key: string, fallback: T): [T, (value: T) => void] {
  const [value, setValue] = useState<T>(() => {
    try {
      const raw = localStorage.getItem(key)
      return raw === null ? fallback : (JSON.parse(raw) as T)
    } catch {
      return fallback
    }
  })
  const set = (next: T) => {
    setValue(next)
    try {
      localStorage.setItem(key, JSON.stringify(next))
    } catch {
      // Keep the in-memory value.
    }
  }
  return [value, set]
}
