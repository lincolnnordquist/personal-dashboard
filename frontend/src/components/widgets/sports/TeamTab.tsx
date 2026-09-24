import { Fragment } from 'react'
import { useQuery } from '@apollo/client/react'
import { GET_STANDINGS, type Game, type Team, type TeamSchedule } from '../../../graphql/queries'
import { kickoff } from '../../../format'
import { recordsByTeam, REFRESH_MS, result, sides } from './sportsUtils'
import TeamLogo from './TeamLogo'

export default function TeamTab({ sport, schedule }: { sport: string; schedule: TeamSchedule }) {
  const { data } = useQuery(GET_STANDINGS, { variables: { sport }, pollInterval: REFRESH_MS })
  const records = recordsByTeam(data?.standings)
  const { team, nextGame } = schedule

  return (
    <div className="team-tab">
      <div className="team-header">
        <TeamLogo team={team} size={44} />
        <div>
          <div className="team-name">{team.displayName}</div>
          <div className="muted">
            {[records.get(team.abbreviation) ?? schedule.record, schedule.standingSummary]
              .filter(Boolean)
              .join(' · ')}
          </div>
        </div>
      </div>

      {nextGame && <NextGame game={nextGame} team={team} records={records} />}

      <h3 className="section-label">Schedule</h3>
      <ol className="schedule">
        {schedule.games.map((g, i) => {
          const prevWeek = i > 0 ? schedule.games[i - 1].week : null
          const showBye =
            schedule.byeWeek !== null &&
            g.week !== null &&
            g.week > schedule.byeWeek &&
            (prevWeek === null || prevWeek < schedule.byeWeek)
          return (
            <Fragment key={g.id}>
              {showBye && (
                <li className="schedule-row muted">
                  <span className="schedule-week">W{schedule.byeWeek}</span>
                  <span>Bye</span>
                </li>
              )}
              <ScheduleRow game={g} team={team} isNext={g.id === nextGame?.id} />
            </Fragment>
          )
        })}
      </ol>
    </div>
  )
}

function NextGame({ game, team, records }: { game: Game; team: Team; records: Map<string, string> }) {
  const { us, them, home } = sides(game, team)
  const live = game.status === 'IN_PROGRESS'
  const opponentRecord = them.record ?? records.get(them.team.abbreviation)

  return (
    <div className="next-game">
      <h3 className="section-label">{live ? 'Live' : 'Next game'}</h3>
      <div className="next-game-matchup">
        <span className="muted">{home ? 'vs' : '@'}</span>
        <TeamLogo team={them.team} size={28} />
        <span className="next-game-opponent">{them.team.displayName}</span>
        {opponentRecord && <span className="muted">{opponentRecord}</span>}
      </div>
      <div className="muted">
        {live ? (
          <>
            <span className="live-dot" /> {us.team.abbreviation} {us.score} – {them.team.abbreviation} {them.score} ·{' '}
            {game.statusDetail}
          </>
        ) : (
          [kickoff(game.date, game.timeValid), game.broadcast].filter(Boolean).join(' · ')
        )}
      </div>
    </div>
  )
}

function ScheduleRow({ game, team, isNext }: { game: Game; team: Team; isNext: boolean }) {
  const { us, them, home } = sides(game, team)

  let outcome: React.ReactNode
  switch (game.status) {
    case 'FINAL': {
      const r = result(us, them)
      outcome = <span className={`result result-${r[0]}`}>{r}</span>
      break
    }
    case 'IN_PROGRESS':
      outcome = (
        <span>
          <span className="live-dot" /> {us.score}-{them.score}
        </span>
      )
      break
    case 'POSTPONED':
    case 'CANCELED':
      outcome = <span className="muted">{game.status === 'POSTPONED' ? 'Postponed' : 'Canceled'}</span>
      break
    default:
      outcome = <span className="muted">{kickoff(game.date, game.timeValid)}</span>
  }

  return (
    <li className={isNext ? 'schedule-row next' : 'schedule-row'}>
      <span className="schedule-week muted">W{game.week}</span>
      <span className="schedule-opponent">
        <span className="muted schedule-at">{home ? 'vs' : '@'}</span>
        <TeamLogo team={them.team} />
        {them.team.abbreviation}
      </span>
      <span className="schedule-outcome">{outcome}</span>
    </li>
  )
}
