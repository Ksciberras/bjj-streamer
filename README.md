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

On the first start only, create the initial administrator interactively. The command refuses to run after any user exists and does not accept the password as an argument:

```bash
docker compose run --rm --entrypoint /bootstrap-admin api --email you@example.com
```

Sign in at <http://localhost:5173>. An administrator can create an expiring invitation for another user and copy its one-time link. There is no public registration endpoint.

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

Authentication uses strict SameSite cookies. The session cookie is HTTP-only; state-changing authenticated requests also require the per-session CSRF token. `AUTH_COOKIE_SECURE=true` is mandatory in production. Invitations expire after `INVITATION_TTL`; sessions enforce both idle and absolute expiry. Rate-limit settings are per minute and held in process memory for the initial single-Droplet architecture.

`/healthz` reports process liveness without touching dependencies. `/readyz` returns `200` only when PostgreSQL responds, otherwise `503`.

## Verification

From the repository root:

```bash
go fmt ./...
go vet ./...
go test ./...
TEST_DATABASE_URL='postgres://bjj:bjj@localhost:5432/bjj?sslmode=disable' go test -race ./...
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
