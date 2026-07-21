# BJJ Streaming Platform

Milestone 1 provides the operational foundation for an invite-only BJJ learning platform: a Go API and worker, React web application, PostgreSQL migrations, and a Docker Compose development environment. It intentionally contains no business or authentication features.

## Prerequisites

- Docker with Docker Compose v2 (recommended for the complete environment)
- Go 1.26+ and Node.js 26+ for running checks directly on the host

## Start a clean checkout

```bash
cp .env.example .env
docker compose up --build --wait
```

The web application is at <http://localhost:5173>. API liveness is at <http://localhost:8080/healthz>; readiness is at <http://localhost:8080/readyz>. Compose waits for PostgreSQL, applies every pending migration, then starts the API and worker. Stop the environment with:

```bash
docker compose down
```

To remove development database data as well, explicitly run `docker compose down --volumes`.

## Host development

Start PostgreSQL alone, apply migrations, then run each process in a separate terminal:

```bash
docker compose up -d db
cp .env.example .env
set -a; source .env; set +a
go run ./cmd/migrate up
go run ./cmd/api
go run ./cmd/worker
```

For the frontend:

```bash
cd apps/web
npm ci
npm run dev
```

Migrations are paired, immutable SQL files under `db/migrations`. Create the next numbered `.up.sql` and `.down.sql` pair for each schema change. `go run ./cmd/migrate down` reverses all migrations and is destructive; use it only against disposable development databases.

## Configuration

All backend processes validate configuration before starting. See `.env.example` for defaults. `DATABASE_URL` is required; supported `APP_ENV` values are `development`, `test`, and `production`; durations use Go syntax such as `5s`; and log levels are `debug`, `info`, `warn`, or `error`. Logs are newline-delimited JSON. HTTP logs include an `X-Request-ID`, preserving a reasonable upstream value or generating one.

`/healthz` reports process liveness without touching dependencies. `/readyz` returns `200` only when PostgreSQL responds, otherwise `503`.

## Verification

From the repository root:

```bash
go fmt ./...
go vet ./...
go test ./...
cd apps/web
npm ci
npm run lint
npm test
npm run build
cd ../..
docker compose config
docker compose up --build --wait
curl --fail http://localhost:8080/healthz
curl --fail http://localhost:8080/readyz
docker compose down --volumes
```

CI runs formatting enforcement, Go vet and race-enabled tests, frontend lint/test/build, Compose validation, migrations, and live health/readiness checks.

## Repository layout

- `cmd/api`, `cmd/worker`, `cmd/migrate`: process entry points
- `internal`: shared modular-monolith infrastructure
- `apps/web`: React application
- `db/migrations`: versioned PostgreSQL schema changes
- `deploy`: development container definitions
- `.github/workflows`: continuous integration

