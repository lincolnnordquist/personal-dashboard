import { useEffect, useState } from 'react'

export function FocusToggle({ focused, onToggle }: { focused: boolean; onToggle: () => void }) {
  const label = focused ? 'Show dashboard (F or Esc)' : 'Focus mode (F)'
  return (
    <button className="focus-toggle" onClick={onToggle} aria-label={label} aria-pressed={focused} title={label}>
      <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
        <path
          fill="currentColor"
          d={
            focused
              ? // Grid: bring the dashboard back.
                'M3 3h8v8H3zm2 2v4h4V5zm8-2h8v8h-8zm2 2v4h4V5zM3 13h8v8H3zm2 2v4h4v-4zm8-2h8v8h-8zm2 2v4h4v-4z'
              : // Eye: just watch and listen.
                'M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5M12 17a5 5 0 1 1 0-10 5 5 0 0 1 0 10m0-8a3 3 0 1 0 0 6 3 3 0 0 0 0-6'
          }
        />
      </svg>
    </button>
  )
}

const timeFormat = new Intl.DateTimeFormat(undefined, { hour: 'numeric', minute: '2-digit' })
const dateFormat = new Intl.DateTimeFormat(undefined, { weekday: 'long', month: 'long', day: 'numeric' })

// FocusClock is the large centered clock and date shown in focus mode.
export function FocusClock() {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 1000)
    return () => clearInterval(id)
  }, [])
  return (
    <div className="focus-clock" aria-live="off">
      <time className="focus-time" dateTime={now.toISOString()}>
        {timeFormat.format(now)}
      </time>
      <div className="focus-date">{dateFormat.format(now)}</div>
    </div>
  )
}
