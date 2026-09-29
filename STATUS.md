# Status

_Last updated: 2026-09-24 (desktop)_

## Current focus

Nothing in progress. Next up is the **Twitch channels** widget (backlog item 1).

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
- **Music player:** no video, just clean controls (song title only, no channel; seek bar, prev/play/next, volume, playlist picker, library). On the dashboard it's a glass bar centered at the bottom; in focus mode it sits centered under the clock and date (its secondary row dims when the mouse is idle). It lives in a full-screen `.stage` in `App.tsx` with the focus clock, so switching modes only changes its layout and never interrupts playback. Code is in `frontend/src/components/music/` and `backend/music/`, with `playlists` and `songs` tables (migrations 3–4).
  - **Multiple playlists.** One plays at a time, chosen with a picker on the player (remembered in localStorage). The same song can be in several playlists. Switching playlists while playing crossfades into the new one.
  - **Music library window** (☰ button): a centered window with playlists on the left (create) and the selected playlist's songs on the right (rename, delete with confirmation, add by pasting a YouTube link, remove, click a song to play it). Esc or the backdrop closes it.
  - **Seek bar** with elapsed/total time; it seeks on release, not while dragging.
  - Shuffle with no repeats per round; 5s crossfade at the end of a song, 1.5s on skip/back/click; volume remembered.
  - **Background themes per playlist:** the library has a Background picker for each playlist ("Mix of all themes" or a theme folder). Clicking a playlist in the library also selects it in the player (crossfading the music if playing), so the background switches to its theme and stays after closing; new playlists open in the library without switching the player. The dashboard background crossfades to the selected playlist's theme right away; no theme, or a theme folder missing on this machine, mixes videos from all themes.
  - **Playlists sync between machines as files in `music/playlists/`** (committed): see the decision below.
- **Keyboard shortcuts:** **B** / **Shift+B** skip to the next / previous background scene within the current theme (0.3s crossfade, and rapid presses each count; automatic transitions keep the 2.5s fade); **V** switches the background between *cover* (fill the screen) and *actual size* (real pixel size, centered, edges faded into the page; remembered in localStorage); **F** toggles focus mode, **Esc** leaves it. None of them fire while typing in a field or with the music library open (`frontend/src/lib/keyboard.ts`).
- **Focus mode:** the round button in the top-right corner (or **F**; **Esc** leaves) fades the dashboard out, leaving the video background, a large clock with the date, and the music controls centered under them. It's remembered in localStorage. After 3s without mouse movement the cursor and the toggle button hide. Code: `frontend/src/components/FocusMode.tsx`, `useFocusMode.ts`.
- **Look:** Glance-style three-column layout with labels above the cards; crossfading ambient video background, with **themes**: one folder of videos per theme in `backgrounds/` (currently `backgrounds/zelda/`, 24 MB, committed); translucent "glass" cards. The user added the search bar, greeting, and Zelda font themselves; keep them.

## Decisions and deviations from the spec

- **Ports:** only 7070 is exposed. The frontend calls `/graphql` on its own origin, so there is no CORS setup and no `VITE_API_URL`.
- **Reddit:** anonymous `.json` is blocked (403), so the widget uses RSS feeds, which have no score or comment counts; `score`/`commentCount` are nullable. RSS allows about 1 request per 60s window; the client follows the `x-ratelimit-*` headers, page loads fail fast, and the refresher waits its turn. Setting `REDDIT_CLIENT_ID`/`REDDIT_CLIENT_SECRET` switches to the OAuth JSON API (tested against fakes only; there are no real credentials yet).
- **Sports:** NFL only. The schema differs from the spec: `Game` has nested `home`/`away` sides, a `GameStatus` enum, logos, and records, and there are new `teamSchedule` and `standings` queries. Widget config: `{sport, featuredTeam, favoriteTeams}`.
- **Weather:** `weatherData` takes optional `unit` and `location` arguments (Open-Meteo returns no place names).
- **YouTube:** the Data API key is sent in the `X-Goog-Api-Key` header, never in the URL. Shorts are detected as ≤180s, which also hides short trailers; that's why Nintendo was dropped as a sample. `channelIds` config accepts @handles or UC… IDs. Uses about 2 quota units per channel per hour.
- **Music player:** uses the official YouTube IFrame API with two players that crossfade by ramping `setVolume`. The players are **hidden** (a transparent 200×200 `.music-engine` box). This goes against YouTube's embed terms, which require a visible player; the user chose it knowingly as a personal-use tradeoff (2026-09-28). Browsers keep playing hidden embeds (verified in Chrome). Songs are checked at add time through YouTube's oEmbed endpoint (no key or quota; 401 means embedding is disabled), and playback errors 101/150 are skipped with a notice.
- **Playlist sync:** `music/playlists/<slug>.txt`, one file per playlist, is the source of truth. The folder is bind-mounted into the backend (`PLAYLIST_DIR`). The first line `# Playlist: Name` holds the display name; the slug comes from the name and renaming a playlist renames its file. Any change in the UI rewrites the files. On startup, and within 5s of any change on disk (e.g. a `git pull`), the database is made to match the folder: playlists and songs are added, renamed, and removed. Files are hand-editable: one video ID or link per line, and bare IDs get their titles looked up. The backend writes as root but chowns files to the folder's owner. The old single `music/playlist.txt` was migrated to `playlists/zelda.txt` and deleted. Code: `backend/music/library.go`, `playlist.go`; DB in `backend/db/music.go`.
- **Background themes:** `backgrounds/<theme>/*.mp4|.webm`, with optional `posters/<same-name>.jpg` stills (used for reduced motion). The folder is bind-mounted into nginx (served at `/backgrounds/`) and into the backend (read-only, `BACKGROUNDS_DIR`), so a new theme is just a new folder: no rebuild, and a page refresh picks it up. **Run `scripts/optimize-backgrounds.sh` after adding clips**: it converts to H.264 MP4 at ≤30fps without audio and adds missing posters. It **never changes resolution** (the user adds clips at 1080p): an earlier version downscaled to 720p, which looked soft on the 1080p/1440p monitors, and the user is re-adding the originals. A clip that only needs its audio removed is stream-copied losslessly; files already done are skipped. Measured in headless Chrome (software rendering, a worst case): replacing `filter: brightness()` on the video with a 15% black overlay took the dashboard from ~55% to 29% of one core; leave filters off the video element. The frosted-glass card blur costs about 13% more in that setup. **Theme fonts:** a font file named after the theme folder (`backgrounds/<theme>/<theme>.woff2|.woff|.otf|.ttf`) is loaded by `useThemeFont` and used **only in focus mode** (clock, date, and song) via the `--theme-font` CSS variable; the dashboard, bottom bar, and music library always use the default fonts (system-ui, with the Hylia 'Zelda' font from `frontend/public/fonts/` for the greeting). No font (or the mix) falls back to those defaults. nginx serves otf/ttf with font MIME types. Video, poster, and font URLs carry a `?v=<mtime+size>` version, because nginx lets browsers cache `/backgrounds/` for 7 days: replacing a clip under the same name gets a new URL, so the new video shows after a refresh. The optimizer also refreshes posters older than their video and deletes posters whose video is gone. `backgroundThemes` lists them; a playlist's theme is saved as `# Theme: <folder>` in its playlist file (migration 5 adds `playlists.theme`). Code: `backend/backgrounds/`, `frontend/src/components/Background.tsx`.
- **Docker:** display only, never start/stop, because socket access is root-equivalent and the dashboard has no login.
- **Widget config changes** currently go through the playground: `mutation { updateWidgetConfig(id: N, config: {...}) { config } }`. Use query variables for configs with empty lists; inline `[]` literals are stored as null.

## Setup needed on a new machine

- `cp .env.example .env`, then fill in `YOUTUBE_API_KEY` (Google Cloud → YouTube Data API v3 → API key restricted to that API).
- `docker compose up -d --build`. A fresh database seeds all widgets with defaults, and the music playlists load from `music/playlists/`. Other settings changed through the UI or playground (subreddits, YouTube channels, …) are stored in each machine's own database and are **not** synced by git.
