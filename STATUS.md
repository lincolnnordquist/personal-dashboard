# Status

_Last updated: 2026-09-24 (desktop)_

## Current focus

Nothing in progress. The music player is done; **focus mode** (backlog item 1) pairs naturally with it and is the suggested next step. Confirm with the user.

## Backlog (in order)

1. **Focus ("clear") mode.** A toggle button that hides everything except the video background, the floating music player, and a **large centered clock with the date**. For using the site as ambient music while focusing. Remember the choice across reloads (localStorage).
2. **Twitch channels widget.** Glance-style list: avatar, name, live/offline, game and viewers when live. Goes in the left column under Docker. Needs a Twitch app (client ID + secret) for the Helix API.
3. **GitHub releases widget.** Repo, latest version, and age, like Glance. Goes in the right column under Weather. Works without a token (60 req/hr); an optional token raises the limit.
4. **Widget editor (spec step 6).** Edit each widget's config in the UI: subreddits, YouTube channels, featured team, location, and column/position.
5. **Remaining Go tests (spec step 7).** Weather response parsing, and cache hit/miss/stale logic in `cache/postgres.go` (use a fake `Store`).
6. **Polish (spec step 8).**
7. **Maybe later:** Google Calendar events on the calendar (needs OAuth); NBA in the sports widget (one line in `sportPaths` in `sports.go`).

## Done

- **Infrastructure:** Docker Compose on a single port (7070) through an nginx proxy, auto-restart, versioned migrations.
- **Weather** (Open-Meteo), right column.
- **NFL** (ESPN), center: Seahawks tab (next game, schedule, bye week), Scores (live/final/upcoming, falls back to last week's finals), Standings (AFC/NFC toggle).
- **Reddit**, center: r/nflv2, r/selfhosted.
- **YouTube**, center: horizontal video row with Shorts hidden. Sample channels: @fireship @linustechtips @mkbhd @veritasium @videogamedunkey.
- **Docker** container status, left: display only, grouped by Compose project.
- **Calendar**, left: month grid, ISO week, Seahawks game-day dots (win/loss/upcoming).
- **Zelda music player:** floating card in the bottom-right, mounted in `App.tsx`. Code is in `frontend/src/components/music/` and `backend/music/`, with `playlists` and `songs` tables (migrations 3–4).
  - **Multiple playlists.** One plays at a time, chosen with a picker on the player (remembered in localStorage). The same song can be in several playlists. Switching playlists while playing crossfades into the new one.
  - **Music library window** (☰ button): a centered window with playlists on the left (create) and the selected playlist's songs on the right (rename, delete with confirmation, add by pasting a YouTube link, remove, click a song to play it). Esc or the backdrop closes it.
  - **Seek bar** with elapsed/total time; it seeks on release, not while dragging.
  - Shuffle with no repeats per round; 5s crossfade at the end of a song, 1.5s on skip/back/click; volume remembered.
  - **Playlists sync between machines as files in `music/playlists/`** (committed): see the decision below.
- **Look:** Glance-style three-column layout with labels above the cards; crossfading Zelda ambient video background (`frontend/public/zelda-backgrounds/`, 24 MB, committed); translucent "glass" cards. The user added the search bar, greeting, and Zelda font themselves; keep them.

## Decisions and deviations from the spec

- **Ports:** only 7070 is exposed. The frontend calls `/graphql` on its own origin, so there is no CORS setup and no `VITE_API_URL`.
- **Reddit:** anonymous `.json` is blocked (403), so the widget uses RSS feeds, which have no score or comment counts; `score`/`commentCount` are nullable. RSS allows about 1 request per 60s window; the client follows the `x-ratelimit-*` headers, page loads fail fast, and the refresher waits its turn. Setting `REDDIT_CLIENT_ID`/`REDDIT_CLIENT_SECRET` switches to the OAuth JSON API (tested against fakes only; there are no real credentials yet).
- **Sports:** NFL only. The schema differs from the spec: `Game` has nested `home`/`away` sides, a `GameStatus` enum, logos, and records, and there are new `teamSchedule` and `standings` queries. Widget config: `{sport, featuredTeam, favoriteTeams}`.
- **Weather:** `weatherData` takes optional `unit` and `location` arguments (Open-Meteo returns no place names).
- **YouTube:** the Data API key is sent in the `X-Goog-Api-Key` header, never in the URL. Shorts are detected as ≤180s, which also hides short trailers; that's why Nintendo was dropped as a sample. `channelIds` config accepts @handles or UC… IDs. Uses about 2 quota units per channel per hour.
- **Music player:** uses the official YouTube IFrame API with two players that crossfade by ramping `setVolume`. YouTube requires the player to stay visible and at least 200×200, so the card shows a 340×200 video. Songs are checked at add time through YouTube's oEmbed endpoint (no key or quota; 401 means embedding is disabled), and playback errors 101/150 are skipped with a notice.
- **Playlist sync:** `music/playlists/<slug>.txt`, one file per playlist, is the source of truth. The folder is bind-mounted into the backend (`PLAYLIST_DIR`). The first line `# Playlist: Name` holds the display name; the slug comes from the name and renaming a playlist renames its file. Any change in the UI rewrites the files. On startup, and within 5s of any change on disk (e.g. a `git pull`), the database is made to match the folder: playlists and songs are added, renamed, and removed. Files are hand-editable: one video ID or link per line, and bare IDs get their titles looked up. The backend writes as root but chowns files to the folder's owner. The old single `music/playlist.txt` was migrated to `playlists/zelda.txt` and deleted. Code: `backend/music/library.go`, `playlist.go`; DB in `backend/db/music.go`.
- **Docker:** display only, never start/stop, because socket access is root-equivalent and the dashboard has no login.
- **Widget config changes** currently go through the playground: `mutation { updateWidgetConfig(id: N, config: {...}) { config } }`. Use query variables for configs with empty lists; inline `[]` literals are stored as null.

## Setup needed on a new machine

- `cp .env.example .env`, then fill in `YOUTUBE_API_KEY` (Google Cloud → YouTube Data API v3 → API key restricted to that API).
- `docker compose up -d --build`. A fresh database seeds all widgets with defaults, and the music playlists load from `music/playlists/`. Other settings changed through the UI or playground (subreddits, YouTube channels, …) are stored in each machine's own database and are **not** synced by git.
