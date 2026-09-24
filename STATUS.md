# Status

_Last updated: 2026-09-24 (desktop)_

## Current focus

Nothing in progress. Next up is the **Twitch channels** widget (see backlog).

## Backlog (in order)

1. **Twitch channels widget.** Glance-style list: avatar, name, live/offline, game and viewers when live. Goes in the left column under Docker. Needs a Twitch app (client ID + secret) for the Helix API.
2. **GitHub releases widget.** Repo, latest version, and age, like Glance. Goes in the right column under Weather. Works without a token (60 req/hr); an optional token raises the limit.
3. **Widget editor (spec step 6).** Edit each widget's config in the UI: subreddits, YouTube channels, featured team, location, and column/position.
4. **Remaining Go tests (spec step 7).** Weather response parsing, and cache hit/miss/stale logic in `cache/postgres.go` (use a fake `Store`).
5. **Polish (spec step 8).**
6. **Maybe later:** Google Calendar events on the calendar (needs OAuth); NBA in the sports widget (one line in `sportPaths` in `sports.go`).

## Done

- **Infrastructure:** Docker Compose on a single port (7070) through an nginx proxy, auto-restart, versioned migrations.
- **Weather** (Open-Meteo), right column.
- **NFL** (ESPN), center: Seahawks tab (next game, schedule, bye week), Scores (live/final/upcoming, falls back to last week's finals), Standings (AFC/NFC toggle).
- **Reddit**, center: r/nflv2, r/selfhosted.
- **YouTube**, center: horizontal video row with Shorts hidden. Sample channels: @fireship @linustechtips @mkbhd @veritasium @videogamedunkey.
- **Docker** container status, left: display only, grouped by Compose project.
- **Calendar**, left: month grid, ISO week, Seahawks game-day dots (win/loss/upcoming).
- **Look:** Glance-style three-column layout with labels above the cards; crossfading Zelda ambient video background (`frontend/public/zelda-backgrounds/`, 24 MB, committed); translucent "glass" cards. The user added the search bar, greeting, and Zelda font themselves; keep them.

## Decisions and deviations from the spec

- **Ports:** only 7070 is exposed. The frontend calls `/graphql` on its own origin, so there is no CORS setup and no `VITE_API_URL`.
- **Reddit:** anonymous `.json` is blocked (403), so the widget uses RSS feeds, which have no score or comment counts; `score`/`commentCount` are nullable. RSS allows about 1 request per 60s window; the client follows the `x-ratelimit-*` headers, page loads fail fast, and the refresher waits its turn. Setting `REDDIT_CLIENT_ID`/`REDDIT_CLIENT_SECRET` switches to the OAuth JSON API (tested against fakes only; there are no real credentials yet).
- **Sports:** NFL only. The schema differs from the spec: `Game` has nested `home`/`away` sides, a `GameStatus` enum, logos, and records, and there are new `teamSchedule` and `standings` queries. Widget config: `{sport, featuredTeam, favoriteTeams}`.
- **Weather:** `weatherData` takes optional `unit` and `location` arguments (Open-Meteo returns no place names).
- **YouTube:** the Data API key is sent in the `X-Goog-Api-Key` header, never in the URL. Shorts are detected as ≤180s, which also hides short trailers; that's why Nintendo was dropped as a sample. `channelIds` config accepts @handles or UC… IDs. Uses about 2 quota units per channel per hour.
- **Docker:** display only, never start/stop, because socket access is root-equivalent and the dashboard has no login.
- **Widget config changes** currently go through the playground: `mutation { updateWidgetConfig(id: N, config: {...}) { config } }`. Use query variables for configs with empty lists; inline `[]` literals are stored as null.

## Setup needed on a new machine

- `cp .env.example .env`, then fill in `YOUTUBE_API_KEY` (Google Cloud → YouTube Data API v3 → API key restricted to that API).
- `docker compose up -d --build`. A fresh database seeds all widgets with defaults. Settings changed through the UI or playground are stored in each machine's own database; they are **not** synced by git.
