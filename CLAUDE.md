# Personal Dashboard

Self-hosted, Glance-inspired dashboard: Go + gqlgen GraphQL backend, React + TypeScript (Vite, Apollo Client 4) frontend, PostgreSQL for widget config and API caching, all run with Docker Compose. `dashboard_spec.md` is the original spec; the project has since deviated from it in places (see STATUS.md).

This project is worked on from two machines (desktop and laptop). **STATUS.md is the shared memory between them**, imported below:

@STATUS.md

## Working agreements

- **Never run git write commands** (commit, push, pull, checkout, restore, stash…). The user does all git work. At a good checkpoint, say what is ready to commit and suggest a message. Read-only git (status, diff, log) is fine.
- **Keep STATUS.md current.** When a feature is finished, a decision is made, or the plan changes, update it in the same change. Keep it short: it is loaded into every session.
- Ask clarifying questions before starting a feature when there are real design choices; the user likes to decide those.
- Verify UI changes in a real browser (headless Chrome screenshots) and backend changes against live APIs, not just tests.
- Never print secrets from `.env`; check them by length or with a test call.

## Running it

- `docker compose up -d --build` from the repo root. Dashboard: http://localhost:7070 (set `DASHBOARD_PORT` in `.env` to change). GraphQL playground: http://localhost:7070/playground.
- Only the frontend publishes a port. nginx (`frontend/nginx.conf`) proxies `/graphql` and `/playground` to the backend; Postgres is reachable only inside Compose: `docker compose exec postgres psql -U dashboard`.
- All services use `restart: unless-stopped` and Docker starts at boot, so the dashboard comes up automatically.
- Secrets live in `.env` (gitignored); `.env.example` is the template.
- Content folders are bind-mounted, not baked into images: `music/playlists/` (the backend reads and writes it) and `backgrounds/<theme>/` (served by nginx, read by the backend). Changes there need no rebuild. After adding background videos, run `scripts/optimize-backgrounds.sh`.
- Backend: `cd backend && go vet ./... && go test ./...`. After editing `graph/schema.graphqls`, run `go tool gqlgen generate`.
- Frontend: `cd frontend && npm run build && npx oxlint`. `npm run dev` serves on :5173 and proxies `/graphql` to a backend on :8080 (`go run .`).

## How the code is organized

- `backend/widgets/<name>.go`: one file per data source (client, response parsing, config parsing), with `<name>_test.go` using real API responses saved in `widgets/testdata/`. Go types here are autobound by gqlgen to the GraphQL types of the same name (see `gqlgen.yml`).
- `backend/cache/postgres.go`: `GetOrFetch` does cache hit/miss/expiry and serves stale data when the upstream fails. Every cached widget goes through it.
- `backend/refresh.go`: background refreshers, one per cached widget, run at each widget's TTL.
- `backend/graph/schema.resolvers.go`: resolvers, kept thin. A failure in one item of a list (one subreddit, one channel) goes to `graphql.AddError` and the rest still return.
- `backend/db/migrate.go`: numbered migrations recorded in `schema_migrations`. **Append new migrations; never edit a shipped one.** Seeding runs only on an empty `widget_config`.
- `frontend/src/components/widgets/<Name>Widget.tsx`: one component per widget type, registered in `components/Grid.tsx`. `WidgetCard` renders the label above the card; tabbed widgets pass `<Tabs variant="label">` as the header.
- Widgets are placed by `widget_config.layout_column` (left / center / right) and `position` within the column.
- Adding a widget touches: `widgets/<name>.go` and its test, `schema.graphqls` (then generate), the resolver, `refresh.go` and `main.go` if cached, a migration to add its row to existing databases, the seed in `defaultWidgets`, `queries.ts`, the component, and `Grid.tsx`.
