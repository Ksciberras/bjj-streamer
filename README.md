# RollStudy

RollStudy is a private BJJ video study workspace at [rollstudy.online](https://rollstudy.online). It implements the functional MVP described in `AGENTS.md`: secure accounts, searchable direct MP4 uploads, authorized playback, per-user resume progress, and private timestamped notes.

Production deployment targets one DigitalOcean Droplet behind Caddy with PostgreSQL on a private Docker network and media in a private Spaces bucket. See `deploy/README.md`.

## Prerequisites

- Docker with Docker Compose v2
- Go 1.26+ and Node.js 26+ for host-side checks

## Start locally

```bash
cp .env.example .env
docker compose up --build --wait
```

The web application is at <http://localhost:5173>. API liveness is at <http://localhost:8080/healthz>; readiness is at <http://localhost:8080/readyz>. MinIO runs privately for local uploads, with its development console at <http://localhost:9001>.

Bootstrap the first administrator once, using an interactive terminal:

```bash
docker compose run --rm --entrypoint /bootstrap-admin api --email you@example.com
```

Sign in through the browser. Administrators can create known user accounts, assign `admin`, `instructor`, or `student` roles, disable accounts, and reset passwords. Password resets and authorization changes revoke the affected user’s active sessions.

Administrators and instructors can upload browser-compatible `.mp4` files up to 5 GiB. The browser uploads bytes directly to object storage; they do not pass through the Go API. Students cannot request uploads. Shared ready videos appear to every authenticated user, while private videos appear only to their uploader and administrators. `personal_purchase` videos must be private.

Select **Watch** to request an authorized short-lived playback URL. The native browser player resumes from your saved position, saves periodically and when paused or left, and supports private timestamped notes. Selecting a note seeks the player to that timestamp. Progress and notes are isolated per user.

Stop the application without deleting PostgreSQL data:

```bash
docker compose down
```

Removing the volume is destructive and should only be done for disposable development data:

```bash
docker compose down --volumes
```

## Host development

```bash
docker compose up -d db
set -a; source .env; set +a
go run ./cmd/migrate up
go run ./cmd/api
```

In another terminal:

```bash
cd apps/web
npm ci
npm run dev
```

## Configuration

`.env.example` contains the local defaults. `DATABASE_URL` is required. Production configuration requires `AUTH_COOKIE_SECURE=true` and explicit object-storage settings.

For DigitalOcean Spaces, configure:

```text
OBJECT_ENDPOINT=https://REGION.digitaloceanspaces.com
OBJECT_PUBLIC_ENDPOINT=https://REGION.digitaloceanspaces.com
OBJECT_REGION=REGION
OBJECT_BUCKET=YOUR_PRIVATE_BUCKET
OBJECT_ACCESS_KEY=YOUR_LEAST_PRIVILEGE_KEY
OBJECT_SECRET_KEY=YOUR_SECRET
OBJECT_PATH_STYLE=false
UPLOAD_URL_TTL=15m
```

Keep the bucket private and configure its CORS policy to allow `PUT` from the application origin with the `Content-Type` header. Credentials belong only in the API environment. Browser compatibility is the uploader's responsibility; the MVP validates the `.mp4` extension, `video/mp4` MIME type, declared size, and stored object metadata but does not inspect codecs or transcode files.

Authentication uses Argon2id passwords, random server-side sessions stored only as token hashes, strict SameSite cookies, CSRF tokens for state-changing authenticated requests, and an in-process login limiter. The limiter resets when the API restarts and is suitable only for the single-Droplet MVP.

The database still contains the older invitation, personal/shared library, membership, and append-only audit structures. They are retained for migration and stored-data compatibility. Invitation endpoints are inactive, and library/audit management is no longer part of the primary frontend journey. Do not edit migrations `000001` through `000003`; use a new migration for future schema changes.

## Verification

From the repository root:

```bash
test -z "$(gofmt -l .)"
go vet ./cmd/... ./internal/...
go test ./cmd/... ./internal/...
TEST_DATABASE_URL='postgres://bjj:bjj@localhost:5432/bjj?sslmode=disable' go test -race ./cmd/... ./internal/...
TEST_OBJECT_STORAGE=1 go test -v ./internal/objectstorage
cd apps/web
npm ci
npm run lint
npm test
npm run build
cd ../..
docker compose config --quiet
docker compose up --build --wait
curl --fail http://localhost:8080/healthz
curl --fail http://localhost:8080/readyz
docker compose down
```

## Repository layout

- `cmd/api`: HTTP API
- `cmd/bootstrap-admin`: first-admin bootstrap command
- `cmd/migrate`: migration runner
- `internal`: authentication, authorization, users, videos, learning data, object storage, compatibility services, and infrastructure
- `apps/web`: React application
- `db/migrations`: immutable versioned PostgreSQL migrations
- `deploy`: container definitions

## Production deployment

Follow [deploy/README.md](deploy/README.md). Deployment, firewall, DNS, Spaces privacy, monitoring, and a real backup restore require manual verification on the target DigitalOcean resources.
