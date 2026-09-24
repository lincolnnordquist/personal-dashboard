import { useEffect, useState } from 'react'
import { useQuery } from '@apollo/client/react'
import { GET_TEAM_SCHEDULE, type Game, type Team } from '../../graphql/queries'
import { dayKey, gameDayKey, kickoff } from '../../format'
import type { WidgetProps } from '../Grid'
import WidgetCard from '../WidgetCard'
import { REFRESH_MS, result, sides } from './sports/sportsUtils'

const WEEKDAYS = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su']
const monthFormat = new Intl.DateTimeFormat(undefined, { month: 'long' })

export default function CalendarWidget({ config }: WidgetProps) {
  const sport = typeof config.sport === 'string' ? config.sport : 'nfl'
  const team = typeof config.team === 'string' ? config.team : ''

  const today = useToday()
  const [offset, setOffset] = useState(0) // months away from the current month
  const view = new Date(today.getFullYear(), today.getMonth() + offset, 1)

  // Mark the team's game days. The NFL widget runs the same query, so Apollo shares the result.
  const { data } = useQuery(GET_TEAM_SCHEDULE, {
    variables: { sport, teamId: team },
    skip: !team,
    pollInterval: REFRESH_MS,
  })
  const schedule = data?.teamSchedule
  const games = new Map<string, Game>()
  for (const g of schedule?.games ?? []) games.set(gameDayKey(g.date, g.timeValid), g)

  const week = isoWeek(today)
  const todayKey = dayKey(today)

  return (
    <WidgetCard title="Calendar">
      <div className="calendar-header">
        <button
          className="calendar-month"
          onClick={() => setOffset(0)}
          title={offset === 0 ? undefined : 'Back to this month'}
        >
          {monthFormat.format(view)}
          {view.getFullYear() !== today.getFullYear() && ` ${view.getFullYear()}`}
        </button>
        <span className="calendar-week muted">
          {offset === 0 ? `Week ${week.week} · ${week.year}` : view.getFullYear()}
        </span>
        <span className="calendar-nav">
          <button onClick={() => setOffset(offset - 1)} aria-label="Previous month">
            ‹
          </button>
          <button onClick={() => setOffset(offset + 1)} aria-label="Next month">
            ›
          </button>
        </span>
      </div>

      <div className="calendar-grid">
        {WEEKDAYS.map((d) => (
          <div key={d} className="calendar-weekday">
            {d}
          </div>
        ))}
        {monthDays(view).map((day) => {
          const key = dayKey(day)
          const game = games.get(key)
          const classes = ['calendar-day']
          if (day.getMonth() !== view.getMonth()) classes.push('other-month')
          if (key === todayKey) classes.push('today')
          return (
            <div key={key} className={classes.join(' ')} title={game && schedule ? gameLabel(game, schedule.team) : undefined}>
              {day.getDate()}
              {game && schedule && <span className={`game-dot ${gameTone(game, schedule.team)}`} />}
            </div>
          )
        })}
      </div>
    </WidgetCard>
  )
}

// useToday returns the current date, refreshed each minute so "today" rolls over at midnight.
function useToday(): Date {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 60 * 1000)
    return () => clearInterval(id)
  }, [])
  return now
}

// monthDays returns every day shown for a month: whole weeks, Monday first.
function monthDays(firstOfMonth: Date): Date[] {
  const year = firstOfMonth.getFullYear()
  const month = firstOfMonth.getMonth()
  const leading = (firstOfMonth.getDay() + 6) % 7 // days shown from the previous month
  const daysInMonth = new Date(year, month + 1, 0).getDate()
  const total = Math.ceil((leading + daysInMonth) / 7) * 7
  return Array.from({ length: total }, (_, i) => new Date(year, month, 1 - leading + i))
}

// isoWeek returns the ISO 8601 week number and the year that week belongs to.
function isoWeek(date: Date): { week: number; year: number } {
  const d = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()))
  // ISO weeks belong to the year of their Thursday.
  d.setUTCDate(d.getUTCDate() + 4 - (d.getUTCDay() || 7))
  const yearStart = Date.UTC(d.getUTCFullYear(), 0, 1)
  const week = Math.ceil(((d.getTime() - yearStart) / 86400000 + 1) / 7)
  return { week, year: d.getUTCFullYear() }
}

function gameLabel(game: Game, team: Team): string {
  const { us, them, home } = sides(game, team)
  const matchup = `${home ? 'vs' : '@'} ${them.team.displayName}`
  switch (game.status) {
    case 'FINAL':
      return `${matchup} · ${result(us, them)}`
    case 'IN_PROGRESS':
      return `${matchup} · Live ${us.score}-${them.score}`
    case 'POSTPONED':
    case 'CANCELED':
      return `${matchup} · ${game.status === 'POSTPONED' ? 'Postponed' : 'Canceled'}`
    default:
      return `${matchup} · ${kickoff(game.date, game.timeValid)}`
  }
}

function gameTone(game: Game, team: Team): 'win' | 'loss' | 'tie' | 'upcoming' {
  if (game.status !== 'FINAL') return 'upcoming'
  const { us, them } = sides(game, team)
  const letter = result(us, them)[0]
  return letter === 'W' ? 'win' : letter === 'L' ? 'loss' : 'tie'
}
