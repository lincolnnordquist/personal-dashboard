import { useState } from 'react'
import { useQuery } from '@apollo/client/react'
import { GET_TEAM_SCHEDULE } from '../../graphql/queries'
import type { WidgetProps } from '../Grid'
import Tabs from '../Tabs'
import WidgetCard from '../WidgetCard'
import ScoresTab from './sports/ScoresTab'
import StandingsTab from './sports/StandingsTab'
import TeamTab from './sports/TeamTab'
import { REFRESH_MS } from './sports/sportsUtils'

type TabId = 'team' | 'scores' | 'standings'

export default function SportsWidget({ config }: WidgetProps) {
  const sport = typeof config.sport === 'string' ? config.sport : 'nfl'
  const featured = typeof config.featuredTeam === 'string' ? config.featuredTeam.toUpperCase() : ''
  const favorites = Array.isArray(config.favoriteTeams)
    ? config.favoriteTeams.filter((t): t is string => typeof t === 'string').map((t) => t.toUpperCase())
    : []
  // Games and standings rows for these team abbreviations are highlighted.
  const highlight = new Set([featured, ...favorites].filter(Boolean))

  const [active, setActive] = useState<TabId>(featured ? 'team' : 'scores')

  const schedule = useQuery(GET_TEAM_SCHEDULE, {
    variables: { sport, teamId: featured },
    skip: !featured,
    pollInterval: REFRESH_MS,
  })
  const teamLabel = schedule.data?.teamSchedule?.team.shortName ?? featured

  const tabs = [
    ...(featured ? [{ id: 'team' as const, label: teamLabel }] : []),
    { id: 'scores' as const, label: 'Scores' },
    { id: 'standings' as const, label: 'Standings' },
  ]

  return (
    <WidgetCard title={sport.toUpperCase()}>
      <Tabs tabs={tabs} active={active} onChange={setActive} />
      <div className="tab-panel">
        {active === 'team' &&
          (schedule.error ? (
            <p className="error">{schedule.error.message}</p>
          ) : schedule.data?.teamSchedule ? (
            <TeamTab sport={sport} schedule={schedule.data.teamSchedule} />
          ) : (
            <p className="muted">Loading…</p>
          ))}
        {active === 'scores' && <ScoresTab sport={sport} highlight={highlight} />}
        {active === 'standings' && <StandingsTab sport={sport} featured={featured} highlight={highlight} />}
      </div>
    </WidgetCard>
  )
}
