import type { Game, GameTeam, StandingsConference, Team } from '../../../graphql/queries'

export const REFRESH_MS = 5 * 60 * 1000

export function isHighlighted(game: Game, highlight: Set<string>): boolean {
  return highlight.has(game.home.team.abbreviation) || highlight.has(game.away.team.abbreviation)
}

// sides returns a game from one team's point of view.
export function sides(game: Game, team: Team): { us: GameTeam; them: GameTeam; home: boolean } {
  const home = game.home.team.id === team.id
  return home ? { us: game.home, them: game.away, home } : { us: game.away, them: game.home, home }
}

// result formats a finished game for one side, e.g. "W 31-7".
export function result(us: GameTeam, them: GameTeam): string {
  const ours = us.score ?? 0
  const theirs = them.score ?? 0
  const letter = ours > theirs ? 'W' : ours < theirs ? 'L' : 'T'
  return `${letter} ${ours}-${theirs}`
}

// recordsByTeam maps team abbreviations to "W-L" or "W-L-T" from standings. Team schedules
// only include records for games already played, so upcoming opponents come from here.
export function recordsByTeam(conferences: StandingsConference[] | undefined): Map<string, string> {
  const records = new Map<string, string>()
  for (const conf of conferences ?? []) {
    for (const div of conf.divisions) {
      for (const e of div.teams) {
        records.set(e.team.abbreviation, formatRecord(e.wins, e.losses, e.ties))
      }
    }
  }
  return records
}

export function formatRecord(wins: number, losses: number, ties: number): string {
  return ties > 0 ? `${wins}-${losses}-${ties}` : `${wins}-${losses}`
}
