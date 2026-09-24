import { useState } from 'react'
import { useQuery } from '@apollo/client/react'
import { GET_STANDINGS, type StandingsConference } from '../../../graphql/queries'
import Tabs from '../../Tabs'
import { formatRecord, REFRESH_MS } from './sportsUtils'
import TeamLogo from './TeamLogo'

export default function StandingsTab({
  sport,
  featured,
  highlight,
}: {
  sport: string
  featured: string
  highlight: Set<string>
}) {
  const { data, loading, error } = useQuery(GET_STANDINGS, { variables: { sport }, pollInterval: REFRESH_MS })
  const conferences = data?.standings ?? []
  const [picked, setPicked] = useState<string | null>(null)

  if (error) return <p className="error">{error.message}</p>
  if (loading && conferences.length === 0) return <p className="muted">Loading…</p>
  if (conferences.length === 0) return null

  // Until the viewer picks one, show the featured team's conference.
  const active = picked ?? conferenceOf(conferences, featured) ?? conferences[0].abbreviation
  const conference = conferences.find((c) => c.abbreviation === active) ?? conferences[0]

  return (
    <div>
      <Tabs
        tabs={conferences.map((c) => ({ id: c.abbreviation, label: c.abbreviation }))}
        active={conference.abbreviation}
        onChange={setPicked}
      />
      {conference.divisions.map((div) => (
        <table key={div.name} className="standings">
          <thead>
            <tr>
              <th className="standings-team">{div.name}</th>
              <th>W-L</th>
              <th>PCT</th>
              <th>DIFF</th>
              <th>STRK</th>
            </tr>
          </thead>
          <tbody>
            {div.teams.map((e) => (
              <tr key={e.team.id} className={highlight.has(e.team.abbreviation) ? 'highlight' : undefined}>
                <td className="standings-team">
                  <span className="standings-team-cell">
                    <TeamLogo team={e.team} size={18} />
                    {e.team.abbreviation}
                  </span>
                </td>
                <td>{formatRecord(e.wins, e.losses, e.ties)}</td>
                <td>{e.winPercent}</td>
                <td>{e.pointDifferential}</td>
                <td>{e.streak}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ))}
    </div>
  )
}

function conferenceOf(conferences: StandingsConference[], team: string): string | undefined {
  return conferences.find((c) => c.divisions.some((d) => d.teams.some((e) => e.team.abbreviation === team)))
    ?.abbreviation
}
