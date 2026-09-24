import { useQuery } from '@apollo/client/react'
import { GET_SCOREBOARD, type Game, type GameTeam } from '../../../graphql/queries'
import { kickoff } from '../../../format'
import { isHighlighted, REFRESH_MS } from './sportsUtils'
import TeamLogo from './TeamLogo'

export default function ScoresTab({ sport, highlight }: { sport: string; highlight: Set<string> }) {
  const { data, loading, error } = useQuery(GET_SCOREBOARD, { variables: { sport }, pollInterval: REFRESH_MS })
  const board = data?.sportsData

  if (error) return <p className="error">{error.message}</p>
  if (loading && !board) return <p className="muted">Loading…</p>
  if (!board) return null

  // Highlighted teams' games go first within each section.
  const favoritesFirst = (games: Game[]) =>
    [...games].sort((a, b) => Number(isHighlighted(b, highlight)) - Number(isHighlighted(a, highlight)))
  const live = board.recentGames.filter((g) => g.status === 'IN_PROGRESS')
  const final = board.recentGames.filter((g) => g.status === 'FINAL')
  const finalWeek = final[0]?.week

  return (
    <div>
      <Section label="Live" games={favoritesFirst(live)} highlight={highlight} />
      <Section label={finalWeek ? `Final · Week ${finalWeek}` : 'Final'} games={favoritesFirst(final)} highlight={highlight} />
      <Section
        label={board.week ? `Upcoming · Week ${board.week}` : 'Upcoming'}
        games={favoritesFirst(board.upcomingGames)}
        highlight={highlight}
      />
      {board.recentGames.length + board.upcomingGames.length === 0 && <p className="muted">No games this week.</p>}
    </div>
  )
}

function Section({ label, games, highlight }: { label: string; games: Game[]; highlight: Set<string> }) {
  if (games.length === 0) return null
  return (
    <section>
      <h3 className="section-label">{label}</h3>
      <ul className="games">
        {games.map((g) => (
          <GameRow key={g.id} game={g} highlighted={isHighlighted(g, highlight)} />
        ))}
      </ul>
    </section>
  )
}

function GameRow({ game, highlighted }: { game: Game; highlighted: boolean }) {
  let status: React.ReactNode
  if (game.status === 'IN_PROGRESS') {
    status = (
      <>
        <span className="live-dot" /> {game.statusDetail}
      </>
    )
  } else if (game.status === 'FINAL') {
    status = game.statusDetail || 'Final'
  } else {
    status = kickoff(game.date, game.timeValid)
  }

  return (
    <li className={highlighted ? 'game highlight' : 'game'}>
      <div className="game-teams">
        <TeamLine side={game.away} game={game} />
        <TeamLine side={game.home} game={game} />
      </div>
      <div className="game-status muted">
        <div>{status}</div>
        {game.status === 'SCHEDULED' && game.broadcast && <div>{game.broadcast}</div>}
      </div>
    </li>
  )
}

function TeamLine({ side, game }: { side: GameTeam; game: Game }) {
  const lost = game.status === 'FINAL' && side.winner === false
  return (
    <div className={lost ? 'team-line lost' : 'team-line'}>
      <TeamLogo team={side.team} />
      <span className="team-abbr">{side.team.abbreviation}</span>
      {game.status === 'SCHEDULED' && side.record && <span className="team-record muted">{side.record}</span>}
      <span className="team-score">{side.score ?? ''}</span>
    </div>
  )
}
