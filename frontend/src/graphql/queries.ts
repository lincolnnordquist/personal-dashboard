import { gql, type TypedDocumentNode } from '@apollo/client'

export interface WidgetConfig {
  id: number
  widgetType: string
  config: Record<string, unknown>
  position: number
  column: 'left' | 'center' | 'right'
  enabled: boolean
}

export const GET_WIDGETS: TypedDocumentNode<{ widgets: WidgetConfig[] }> = gql`
  query GetWidgets {
    widgets {
      id
      widgetType
      config
      position
      column
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

export interface YoutubeVideo {
  title: string
  videoId: string
  publishedAt: string
  thumbnailUrl: string
  durationSeconds: number
  viewCount: number | null
}

export interface YoutubeChannel {
  channelId: string
  channelName: string
  handle: string | null
  videos: YoutubeVideo[]
}

export const GET_YOUTUBE: TypedDocumentNode<{ youtubeData: YoutubeChannel[] }, { channelIds: string[] }> = gql`
  query GetYoutube($channelIds: [String!]!) {
    youtubeData(channelIds: $channelIds) {
      channelId
      channelName
      handle
      videos {
        title
        videoId
        publishedAt
        thumbnailUrl
        durationSeconds
        viewCount
      }
    }
  }
`

export interface Song {
  id: number
  playlistId: number
  videoId: string
  title: string
  channelName: string
  thumbnailUrl: string
  addedAt: string
}

export interface Playlist {
  id: number
  name: string
  // Background theme folder; null mixes all themes.
  theme: string | null
  songs: Song[]
}

const SONG_FIELDS = gql`
  fragment SongFields on Song {
    id
    playlistId
    videoId
    title
    channelName
    thumbnailUrl
    addedAt
  }
`

export const GET_PLAYLISTS: TypedDocumentNode<{ playlists: Playlist[] }> = gql`
  ${SONG_FIELDS}
  query GetPlaylists {
    playlists {
      id
      name
      theme
      songs {
        ...SongFields
      }
    }
  }
`

export const CREATE_PLAYLIST: TypedDocumentNode<{ createPlaylist: { id: number; name: string } }, { name: string }> = gql`
  mutation CreatePlaylist($name: String!) {
    createPlaylist(name: $name) {
      id
      name
    }
  }
`

export const RENAME_PLAYLIST: TypedDocumentNode<
  { renamePlaylist: { id: number; name: string } },
  { id: number; name: string }
> = gql`
  mutation RenamePlaylist($id: Int!, $name: String!) {
    renamePlaylist(id: $id, name: $name) {
      id
      name
    }
  }
`

export const DELETE_PLAYLIST: TypedDocumentNode<{ deletePlaylist: boolean }, { id: number }> = gql`
  mutation DeletePlaylist($id: Int!) {
    deletePlaylist(id: $id)
  }
`

export const ADD_SONG: TypedDocumentNode<{ addSong: Song }, { playlistId: number; url: string }> = gql`
  ${SONG_FIELDS}
  mutation AddSong($playlistId: Int!, $url: String!) {
    addSong(playlistId: $playlistId, url: $url) {
      ...SongFields
    }
  }
`

export const REMOVE_SONG: TypedDocumentNode<{ removeSong: boolean }, { id: number }> = gql`
  mutation RemoveSong($id: Int!) {
    removeSong(id: $id)
  }
`

export interface BackgroundVideo {
  name: string
  url: string
  posterUrl: string | null
}

export interface BackgroundTheme {
  name: string
  videos: BackgroundVideo[]
}

export const GET_BACKGROUND_THEMES: TypedDocumentNode<{ backgroundThemes: BackgroundTheme[] }> = gql`
  query GetBackgroundThemes {
    backgroundThemes {
      name
      videos {
        name
        url
        posterUrl
      }
    }
  }
`

export const SET_PLAYLIST_THEME: TypedDocumentNode<
  { setPlaylistTheme: { id: number; theme: string | null } },
  { id: number; theme: string | null }
> = gql`
  mutation SetPlaylistTheme($id: Int!, $theme: String) {
    setPlaylistTheme(id: $id, theme: $theme) {
      id
      theme
    }
  }
`
