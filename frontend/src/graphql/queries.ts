import { gql, type TypedDocumentNode } from '@apollo/client'

export interface WidgetConfig {
  id: number
  widgetType: string
  config: Record<string, unknown>
  position: number
  enabled: boolean
}

export const GET_WIDGETS: TypedDocumentNode<{ widgets: WidgetConfig[] }> = gql`
  query GetWidgets {
    widgets {
      id
      widgetType
      config
      position
      enabled
    }
  }
`

export type TemperatureUnit = 'F' | 'C'

export interface WeatherData {
  temperature: number
  condition: string
  high: number
  low: number
  location: string
}

export const GET_WEATHER: TypedDocumentNode<
  { weatherData: WeatherData | null },
  { lat: number; lon: number; unit: TemperatureUnit; location?: string }
> = gql`
  query GetWeather($lat: Float!, $lon: Float!, $unit: TemperatureUnit, $location: String) {
    weatherData(lat: $lat, lon: $lon, unit: $unit, location: $location) {
      temperature
      condition
      high
      low
      location
    }
  }
`

export interface RedditPost {
  title: string
  score: number | null
  commentCount: number | null
  url: string
  author: string
  publishedAt: string
}

export interface SubredditFeed {
  subreddit: string
  posts: RedditPost[]
}

export const GET_REDDIT: TypedDocumentNode<{ redditData: SubredditFeed[] }, { subreddits: string[] }> = gql`
  query GetReddit($subreddits: [String!]!) {
    redditData(subreddits: $subreddits) {
      subreddit
      posts {
        title
        score
        commentCount
        url
        author
        publishedAt
      }
    }
  }
`

export interface Team {
  id: string
  abbreviation: string
  displayName: string
  shortName: string
  logo: string
  color: string
}

export type GameStatus = 'SCHEDULED' | 'IN_PROGRESS' | 'FINAL' | 'POSTPONED' | 'CANCELED'

export interface GameTeam {
  team: Team
  score: number | null
  winner: boolean | null
  record: string | null
}

export interface Game {
  id: string
  week: number | null
  date: string
  timeValid: boolean
  status: GameStatus
  statusDetail: string
  broadcast: string | null
  home: GameTeam
  away: GameTeam
}

export interface TeamSchedule {
  team: Team
  record: string
  standingSummary: string
  byeWeek: number | null
  nextGame: Game | null
  games: Game[]
}

export interface StandingsEntry {
  team: Team
  wins: number
  losses: number
  ties: number
  winPercent: string
  pointDifferential: string
  streak: string
  playoffSeed: number | null
}

export interface StandingsConference {
  name: string
  abbreviation: string
  divisions: { name: string; teams: StandingsEntry[] }[]
}

const TEAM_FIELDS = gql`
  fragment TeamFields on Team {
    id
    abbreviation
    displayName
    shortName
    logo
    color
  }
`

const GAME_FIELDS = gql`
  ${TEAM_FIELDS}
  fragment GameFields on Game {
    id
    week
    date
    timeValid
    status
    statusDetail
    broadcast
    home {
      team {
        ...TeamFields
      }
      score
      winner
      record
    }
    away {
      team {
        ...TeamFields
      }
      score
      winner
      record
    }
  }
`

export const GET_TEAM_SCHEDULE: TypedDocumentNode<
  { teamSchedule: TeamSchedule | null },
  { sport: string; teamId: string }
> = gql`
  ${GAME_FIELDS}
  query GetTeamSchedule($sport: String!, $teamId: String!) {
    teamSchedule(sport: $sport, teamId: $teamId) {
      team {
        ...TeamFields
      }
      record
      standingSummary
      byeWeek
      nextGame {
        ...GameFields
      }
      games {
        ...GameFields
      }
    }
  }
`

export const GET_SCOREBOARD: TypedDocumentNode<
  { sportsData: { week: number | null; recentGames: Game[]; upcomingGames: Game[] } | null },
  { sport: string }
> = gql`
  ${GAME_FIELDS}
  query GetScoreboard($sport: String!) {
    sportsData(sport: $sport) {
      week
      recentGames {
        ...GameFields
      }
      upcomingGames {
        ...GameFields
      }
    }
  }
`

export const GET_STANDINGS: TypedDocumentNode<{ standings: StandingsConference[] }, { sport: string }> = gql`
  ${TEAM_FIELDS}
  query GetStandings($sport: String!) {
    standings(sport: $sport) {
      name
      abbreviation
      divisions {
        name
        teams {
          team {
            ...TeamFields
          }
          wins
          losses
          ties
          winPercent
          pointDifferential
          streak
          playoffSeed
        }
      }
    }
  }
`

export interface DockerContainer {
  id: string
  name: string
  status: string
  health: string | null
  image: string
  uptime: string
  statusText: string
  project: string | null
  service: string | null
}

export const GET_DOCKER: TypedDocumentNode<{ dockerData: DockerContainer[] }> = gql`
  query GetDocker {
    dockerData {
      id
      name
      status
      health
      image
      uptime
      statusText
      project
      service
    }
  }
`
