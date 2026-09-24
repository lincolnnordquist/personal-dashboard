import type { Team } from '../../../graphql/queries'

export default function TeamLogo({ team, size = 20 }: { team: Team; size?: number }) {
  if (!team.logo) return <span className="team-logo" style={{ width: size, height: size }} />
  return <img className="team-logo" src={team.logo} alt="" width={size} height={size} loading="lazy" />
}
