# Personal Dashboard — Project Specification

## Overview
A self-hosted personal dashboard built in Go, inspired by Glance. Displays configurable widgets in a dark-mode grid layout. Runs locally via Docker Compose with a React frontend consuming a Go/GraphQL backend backed by PostgreSQL.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go (Golang) |
| API | GraphQL (gqlgen library) |
| Frontend | React + TypeScript |
| Database | PostgreSQL |
| Containerization | Docker + Docker Compose |
| Testing | Go standard testing package (`testing`) + testify |

---

## Architecture

```
┌─────────────────────────────────────────────┐
│               Docker Compose                │
│                                             │
│  ┌──────────┐   ┌──────────┐  ┌──────────┐ │
│  │  React   │   │  Go API  │  │Postgres  │ │
│  │ Frontend │──▶│ GraphQL  │──▶│   DB     │ │
│  │ :3000    │   │  :8080   │  │  :5432   │ │
│  └──────────┘   └──────────┘  └──────────┘ │
└─────────────────────────────────────────────┘
```

- React frontend makes GraphQL queries to the Go backend
- Go backend fetches data from external APIs, caches results in PostgreSQL, and serves via GraphQL
- All three services run via `docker compose up`

---

## Project Structure

```
dashboard/
├── backend/
│   ├── main.go
│   ├── graph/
│   │   ├── schema.graphqls
│   │   ├── resolver.go
│   │   └── generated.go        # gqlgen generated
│   ├── widgets/
│   │   ├── weather.go
│   │   ├── sports.go
│   │   ├── reddit.go
│   │   ├── youtube.go
│   │   └── docker.go
│   ├── cache/
│   │   └── postgres.go         # cache read/write helpers
│   ├── db/
│   │   └── migrate.go          # schema migration on startup
│   └── config/
│       └── config.go           # env var loading
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   │   ├── Grid.tsx
│   │   │   ├── widgets/
│   │   │   │   ├── WeatherWidget.tsx
│   │   │   │   ├── SportsWidget.tsx
│   │   │   │   ├── RedditWidget.tsx
│   │   │   │   ├── YouTubeWidget.tsx
│   │   │   │   └── DockerWidget.tsx
│   │   │   └── WidgetEditor.tsx  # UI for configuring widgets
│   │   ├── graphql/
│   │   │   └── queries.ts
│   │   ├── App.tsx
│   │   └── main.tsx
│   └── package.json
├── docker-compose.yml
├── backend/Dockerfile
├── frontend/Dockerfile
└── .env.example
```

---

## Widgets

### 1. Weather
- **API**: Open-Meteo (free, no API key required)
- **Data**: Current temperature, condition, high/low for the day, location name
- **Cache strategy**: PostgreSQL, refreshed every 30 minutes via background goroutine
- **Config**: Location (lat/long), temperature unit (F/C)

### 2. Sports Scores
- **API**: ESPN public API (unofficial, no key required) or SportsData.io free tier
- **Sports**: NFL + NBA
- **Data**: Recent scores/results, upcoming games, standings
- **Cache strategy**: PostgreSQL, refreshed every 15 minutes
- **Config**: Favorite teams to highlight

### 3. Reddit Feed
- **API**: Reddit JSON API (no auth required for public subreddits, append `.json` to any subreddit URL)
- **Data**: Top posts from configured subreddits (title, score, comment count, link)
- **Cache strategy**: PostgreSQL, refreshed every 20 minutes
- **Config**: List of subreddits to display
- **Note**: Live fetch on page load as fallback if cache is stale

### 4. YouTube Channel Uploads
- **API**: YouTube Data API v3 (free tier, requires API key)
- **Data**: Latest videos from configured channels (title, thumbnail, date, link)
- **Cache strategy**: PostgreSQL, refreshed every 60 minutes
- **Config**: List of YouTube channel IDs to follow

### 5. Docker Container Status
- **API**: Docker socket (`/var/run/docker.sock`) via Go Docker SDK
- **Data**: Container name, status (running/stopped/exited), image, uptime
- **Cache strategy**: Live fetch every time (no caching — always needs to be current)
- **Note**: Requires mounting Docker socket in docker-compose.yml

---

## Database Schema (PostgreSQL)

```sql
-- Widget configuration (user preferences, editable via UI)
CREATE TABLE widget_config (
    id SERIAL PRIMARY KEY,
    widget_type VARCHAR(50) NOT NULL,  -- 'weather', 'sports', 'reddit', etc.
    config JSONB NOT NULL,             -- widget-specific settings
    position INT NOT NULL,             -- grid position order
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Cache table for external API responses
CREATE TABLE widget_cache (
    id SERIAL PRIMARY KEY,
    widget_type VARCHAR(50) NOT NULL,
    cache_key VARCHAR(255) NOT NULL,   -- e.g. 'weather_40.7128_-74.0060'
    data JSONB NOT NULL,               -- cached API response
    fetched_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    UNIQUE(widget_type, cache_key)
);
```

---

## GraphQL Schema

```graphql
type Query {
  widgets: [WidgetConfig!]!
  weatherData(lat: Float!, lon: Float!): WeatherData
  sportsData(sport: String!, teamIds: [String!]): SportsData
  redditData(subreddits: [String!]!): [SubredditFeed!]!
  youtubeData(channelIds: [String!]!): [YoutubeChannel!]!
  dockerData: [DockerContainer!]!
}

type Mutation {
  updateWidgetConfig(id: Int!, config: JSON!, position: Int): WidgetConfig!
  toggleWidget(id: Int!, enabled: Boolean!): WidgetConfig!
}

type WidgetConfig {
  id: Int!
  widgetType: String!
  config: JSON!
  position: Int!
  enabled: Boolean!
}

type WeatherData {
  temperature: Float!
  condition: String!
  high: Float!
  low: Float!
  location: String!
}

type SportsData {
  recentGames: [Game!]!
  upcomingGames: [Game!]!
}

type Game {
  homeTeam: String!
  awayTeam: String!
  homeScore: Int
  awayScore: Int
  status: String!
  date: String!
}

type SubredditFeed {
  subreddit: String!
  posts: [RedditPost!]!
}

type RedditPost {
  title: String!
  score: Int!
  commentCount: Int!
  url: String!
  author: String!
}

type YoutubeChannel {
  channelName: String!
  videos: [YoutubeVideo!]!
}

type YoutubeVideo {
  title: String!
  videoId: String!
  publishedAt: String!
  thumbnailUrl: String!
}

type DockerContainer {
  name: String!
  status: String!
  image: String!
  uptime: String!
}
```

---

## Caching Strategy

| Widget | Strategy | Refresh Interval |
|---|---|---|
| Weather | Cache in PostgreSQL | 30 min |
| Sports | Cache in PostgreSQL | 15 min |
| Reddit | Cache in PostgreSQL | 20 min |
| YouTube | Cache in PostgreSQL | 60 min |
| Docker | Live (no cache) | On every request |

Background goroutines in Go refresh each cached widget on its interval. On a cache miss or expired cache, the backend fetches live and updates the cache before responding.

---

## Frontend — React

### Key Components
- **`App.tsx`** — root component, fetches all widget configs, renders grid
- **`Grid.tsx`** — CSS grid layout, maps widget configs to widget components
- **`WeatherWidget.tsx`** — displays weather data
- **`SportsWidget.tsx`** — displays NFL/NBA scores and upcoming games
- **`RedditWidget.tsx`** — displays subreddit post feeds
- **`YouTubeWidget.tsx`** — displays latest channel uploads
- **`DockerWidget.tsx`** — displays container status with color indicators
- **`WidgetEditor.tsx`** — modal/panel for editing widget configuration (subreddits list, channel IDs, favorite teams, location, etc.)

### State management
- Apollo Client for GraphQL queries and mutations
- Local React state for UI (modal open/close, edit mode)

---

## Docker Compose

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: dashboard
      POSTGRES_USER: dashboard
      POSTGRES_PASSWORD: dashboard
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  backend:
    build: ./backend
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://dashboard:dashboard@postgres:5432/dashboard
      YOUTUBE_API_KEY: ${YOUTUBE_API_KEY}
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    depends_on:
      - postgres

  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
    environment:
      VITE_API_URL: http://localhost:8080/graphql
    depends_on:
      - backend

volumes:
  pgdata:
```

---

## Environment Variables

```env
# Required
DATABASE_URL=postgres://dashboard:dashboard@postgres:5432/dashboard
YOUTUBE_API_KEY=your_youtube_api_key_here

# Optional — defaults to St. George, UT
DEFAULT_LAT=37.0965
DEFAULT_LON=-113.5684
DEFAULT_LOCATION_NAME=St. George, UT
```

---

## Unit Tests (Go)

Write tests for the following using Go's `testing` package + `testify`:

- `widgets/weather.go` — test data parsing from Open-Meteo response
- `widgets/reddit.go` — test subreddit post parsing
- `cache/postgres.go` — test cache hit/miss logic
- `widgets/sports.go` — test score parsing and game status logic

Run with: `go test ./...`

---

## Build Order / Suggested Implementation Steps

1. Set up Docker Compose with Postgres + Go service skeleton
2. Set up PostgreSQL schema (run migrations on backend startup)
3. Install and configure gqlgen, define schema, generate boilerplate
4. Build one widget end-to-end first (Weather — no API key needed):
   - Go fetcher → cache → GraphQL resolver → React component
5. Repeat for Reddit (also no API key), Sports, YouTube, Docker
6. Build WidgetEditor UI for configuring widgets
7. Write Go unit tests
8. Polish frontend grid layout and dark mode styling

---

## API Keys Needed

| Service | Key Required | Where to Get |
|---|---|---|
| Open-Meteo | No | Free, no signup |
| Reddit | No | Public JSON API |
| ESPN | No | Unofficial public API |
| YouTube Data API v3 | Yes | Google Cloud Console (free tier) |
| Docker | No | Local socket mount |

---

## Resume / Portfolio Talking Points

- Built a self-hosted personal dashboard in Go (new language)
- Designed and implemented a GraphQL API using gqlgen
- Used PostgreSQL for widget config persistence and API response caching with TTL logic
- Containerized the full stack (Go backend + React frontend + PostgreSQL) with Docker Compose
- Wrote unit tests for Go backend services using the standard testing package
- Integrated 5 external data sources with mixed caching strategies
