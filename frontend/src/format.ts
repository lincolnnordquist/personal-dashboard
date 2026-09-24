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
