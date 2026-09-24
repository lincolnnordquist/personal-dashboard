const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['week', 7 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
]

const relative = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto', style: 'short' })

// timeAgo formats an ISO timestamp relative to now, e.g. "3 hr. ago".
export function timeAgo(iso: string): string {
  const seconds = (Date.now() - new Date(iso).getTime()) / 1000
  for (const [unit, size] of units) {
    if (seconds >= size) return relative.format(-Math.floor(seconds / size), unit)
  }
  return 'just now'
}

// greeting returns a time-of-day salutation for the given name, e.g. "Good afternoon, Lincoln".
export function greeting(name: string): string {
  const hour = new Date().getHours()
  const part = hour < 12 ? 'morning' : hour < 18 ? 'afternoon' : 'evening'
  return `Good ${part}, ${name}`
}

export function compactNumber(n: number): string {
  return Intl.NumberFormat(undefined, { notation: 'compact' }).format(n)
}

const dayFormat = new Intl.DateTimeFormat(undefined, { weekday: 'short', month: 'numeric', day: 'numeric' })
const timeFormat = new Intl.DateTimeFormat(undefined, { hour: 'numeric', minute: '2-digit' })
// ESPN sets unscheduled kickoffs to midnight US Eastern on the game's date.
const placeholderDayFormat = new Intl.DateTimeFormat(undefined, {
  weekday: 'short',
  month: 'numeric',
  day: 'numeric',
  timeZone: 'America/New_York',
})

// kickoff formats a game time in the viewer's time zone, e.g. "Sun 9/27 · 11:00 AM".
export function kickoff(iso: string, timeValid: boolean): string {
  const date = new Date(iso)
  if (!timeValid) return `${placeholderDayFormat.format(date)} · TBD`
  return `${dayFormat.format(date)} · ${timeFormat.format(date)}`
}

// YYYY-MM-DD keys for grouping things by calendar day. en-CA formats dates as YYYY-MM-DD.
const dayKeyFormat = new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit' })
const easternDayKeyFormat = new Intl.DateTimeFormat('en-CA', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  timeZone: 'America/New_York',
})

// dayKey returns the viewer's local calendar day for a date.
export function dayKey(date: Date): string {
  return dayKeyFormat.format(date)
}

// gameDayKey returns the local calendar day a game is played on. Games without a kickoff
// time use a midnight US Eastern placeholder, so their day is read in that time zone.
export function gameDayKey(iso: string, timeValid: boolean): string {
  const date = new Date(iso)
  return timeValid ? dayKeyFormat.format(date) : easternDayKeyFormat.format(date)
}

// videoLength formats seconds as a video length, e.g. "15:06" or "1:02:03".
export function videoLength(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = String(seconds % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`
}
