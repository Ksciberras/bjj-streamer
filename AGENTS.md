# BJJ Streaming MVP — Agent Instructions

## Goal

Build the smallest useful private BJJ instructional platform for 5–20 known
users. The MVP must prove one loop:

```text
admin/instructor uploads MP4 -> user finds it -> watches it -> resumes later ->
adds a timestamped note
```

Do not build a production platform before this loop works and is used.

## Fixed MVP choices

- React + TypeScript + Vite frontend.
- Go HTTP API.
- PostgreSQL.
- DigitalOcean Spaces for private video storage.
- One DigitalOcean Droplet when deployment is ready.
- Docker Compose for local development and deployment.
- Caddy for HTTPS in deployment.
- Invite-only in practice: an admin creates user accounts. There is no public
  registration or email invitation workflow in the MVP.
- Roles: `admin`, `instructor`, and `student`.
- Only admins and instructors can upload videos.
- Admins can manage every video. Instructors can manage only their own videos.
- Every authenticated user can watch shared videos.
- Private videos can be watched only by their uploader and admins.
- Accept browser-compatible `.mp4` files only. Do not add FFmpeg, transcoding,
  HLS, or a background worker in the MVP.
- Use one presigned PUT upload per file. Set a documented maximum file size of
  5 GiB. Multipart and resumable upload are post-MVP.

## Content rules

Every video has:

```text
visibility: shared | private
content_basis: self_created | licensed_for_group | personal_purchase
```

Enforce this backend rule:

```text
personal_purchase -> visibility must be private
```

Do not implement DRM removal, scraping, credential sharing, or public links.
Only self-created or properly licensed material may be shared.

## MVP features

### Accounts

- Bootstrap the first admin from a CLI command or environment variables.
- Admin can create, disable, and reset passwords for known users.
- Users log in with email and password.
- Passwords use Argon2id.
- Authentication uses secure HTTP-only session cookies.
- There is no self-registration, email delivery, password recovery email, MFA,
  OAuth, or organization management in the MVP.

### Video catalog

- Admin or instructor creates a video record.
- Required metadata: title, instructor name, visibility, content basis, and
  object key.
- Optional metadata: instructional/series name, chapter name, description, and
  comma-separated tags.
- Optional custom thumbnail: JPEG, PNG, or WebP up to 5 MiB. When omitted, the
  browser may generate a JPEG from the selected local MP4. Both variants upload
  directly to private object storage with the same authorization floor as the
  video.
- Browse all accessible videos.
- Search title, instructor, instructional name, and tags.
- Admin can edit/archive anything.
- Instructor can edit/archive only videos they uploaded.
- Students have read-only catalog access.

Do not build separate position, technique, volume, alias, or relationship
tables yet. Plain metadata and tags are enough to validate the product.

### Upload

1. Admin or instructor creates an upload request.
2. API checks the role and validates filename, MIME type, size, visibility, and
   content basis.
3. API generates the object key and a short-lived presigned PUT URL.
4. Browser uploads the MP4 directly to Spaces.
5. Browser tells the API that upload finished.
6. API verifies the object exists and marks the video ready.

Video bytes must not pass through the Go API. Students cannot call upload
endpoints. Spaces credentials must never reach the frontend.

### Playback and study

- API checks access and returns a short-lived presigned GET URL.
- Browser plays the MP4 with the native `<video>` element.
- Save playback position periodically and on pause/navigation.
- Resume from the saved position.
- Users can add, edit, delete, and click timestamped private notes.
- Clicking a note seeks the player to that timestamp.
- Progress and notes are always private per user.

Bookmarks, study queues, completion rules, shared notes, transcripts, and
recommendations are post-MVP.

## Minimal data model

Use only the tables needed for the working loop:

```text
users
- id
- email
- password_hash
- role: admin | instructor | student
- disabled_at
- created_at

sessions
- id_hash
- user_id
- expires_at
- created_at

videos
- id
- uploaded_by_user_id
- title
- instructor_name
- instructional_name nullable
- chapter_name nullable
- description
- tags text[] or normalized simple tags
- visibility: shared | private
- content_basis
- object_key
- original_filename
- mime_type
- byte_size
- status: pending_upload | ready | archived
- created_at
- updated_at

playback_progress
- user_id
- video_id
- position_seconds
- updated_at

notes
- id
- user_id
- video_id
- timestamp_seconds
- body
- created_at
- updated_at
```

Prefer database constraints for enum values, non-negative timestamps, and the
personal-purchase visibility rule.

## Minimal architecture

```text
React web app -> Go API -> PostgreSQL
      |             |
      +-------> private DigitalOcean Spaces
```

Suggested repository structure:

```text
apps/web/
cmd/api/
internal/
db/migrations/
deploy/
compose.yaml
README.md
```

Keep the Go application a small modular monolith. Do not add Redis, queues,
workers, microservices, Kubernetes, Terraform, generated API clients, a design
system, or generic repository abstractions.

## Security floor

The MVP is simple, not careless.

- Enforce permissions in the Go API, never only in React.
- Use parameterized database queries.
- Use Argon2id password hashing.
- Store only a hash of each session token in PostgreSQL.
- Use HTTP-only, Secure-in-production, SameSite cookies.
- Add CSRF protection to state-changing cookie-authenticated requests.
- Rate-limit login attempts with a simple in-process limiter. Document that it
  resets on restart and is sufficient only for the single-Droplet MVP.
- Generate object keys on the server.
- Keep the Spaces bucket private.
- Return short-lived signed URLs only after authorization.
- Never log passwords, session tokens, Spaces secrets, or signed URL query
  strings.
- Validate upload MIME type, extension, and declared size. Verification after
  upload is limited in this MVP; document that browser compatibility is the
  uploader's responsibility.

## How agents should work

- Read this file before editing.
- Implement one milestone at a time and stop at its gate.
- Do not add future features or abstractions speculatively.
- Prefer a complete working path over broad scaffolding.
- Keep code organized by cohesive responsibility. Do not accumulate unrelated
  screens, domain logic, transport code, and utilities in one large file.
- Split a module when it has multiple independent reasons to change. Prefer
  feature modules with explicit, narrow interfaces over miscellaneous
  `helpers`, `utils`, or generic framework layers.
- Keep React screen components, reusable UI components, API transport, shared
  types, and formatting functions in separate modules when they are reused or
  independently testable. Keep state close to the feature that owns it.
- Keep Go code inside the existing domain packages. Extract focused helpers or
  services only when doing so clarifies a real responsibility; do not introduce
  generic repositories, dependency-injection frameworks, or one-method
  interfaces merely to reduce file length.
- Favor readable control flow, descriptive names, and small testable functions.
  Avoid compressed one-line components and dense multi-operation statements.
- Refactor incrementally and preserve behavior. A cleanup is not permission to
  change API contracts, authorization, database compatibility, or product
  scope.
- Add tests around authorization and data isolation.
- Use migrations for schema changes.
- Do not edit an already-applied migration; create a new one.
- Update the README with exact commands.
- Run applicable verification and report commands actually run.
- Do not claim Docker or cloud behavior was tested if it was not.

Typical verification:

```bash
go fmt ./...
go vet ./...
go test ./...
npm run lint
npm test
npm run build
docker compose config
```

## Four milestones

### Milestone 1 — Local app with login

Build:

- Go API, React frontend, PostgreSQL, migrations, and Docker Compose.
- Configuration loading.
- Health endpoint.
- Users and sessions tables.
- First-admin bootstrap command.
- Login, logout, current-user endpoint, and protected app shell.
- Minimal tests for password hashing, sessions, and protected routes.
- README with one-command local startup.

Gate:

- A clean checkout starts locally using documented commands.
- The admin can be bootstrapped and can log in through the browser.
- Anonymous users cannot access the app.
- Relevant tests and builds pass.

Stop after this gate. Do not add videos yet.

### Milestone 2 — Catalog and direct MP4 upload

Build:

- Admin user-management screen for creating and disabling known users.
- Videos table and content constraints.
- Video create/edit/archive endpoints and screens.
- Role and ownership checks.
- Presigned PUT upload to a local S3-compatible service in development and
  DigitalOcean Spaces in deployment.
- Upload progress and completion verification.
- Accessible video list and basic search.

Gate:

- Admin and instructor can upload a compatible MP4 directly to object storage.
- Student upload requests are rejected by the API.
- Instructor cannot edit another instructor's video.
- `personal_purchase` cannot be shared.
- All users see shared ready videos; private videos remain private.

Stop after this gate. Do not add transcoding or multipart upload.

### Milestone 3 — Playback, progress, and notes

Build:

- Authorized short-lived playback URLs.
- Native responsive video player.
- Per-user saved and resumed playback position.
- Timestamped private notes with click-to-seek.
- Basic mobile layout.
- Tests proving progress and note isolation.

Gate:

- Two users can watch the same video with independent progress and notes.
- Unauthorized users cannot obtain a playback URL.
- Seeking works in current Safari and Chromium browsers.
- A user can leave, return, resume, and jump to a saved note.

This is the functional MVP. Use it locally before deploying.

### Milestone 4 — Simple deployment and pilot

Build only what is needed to run the proven MVP:

- Production Dockerfiles and Compose file.
- Caddy HTTPS configuration.
- Database backup and restore scripts.
- Deployment README.
- Basic health monitoring instructions.

Manual deployment target:

- One small DigitalOcean Droplet.
- One private Spaces bucket.
- One domain or subdomain.
- PostgreSQL running privately in Docker Compose on the Droplet.

Gate:

- Site works over HTTPS.
- PostgreSQL is not publicly exposed.
- Spaces objects are private.
- A real backup is restored successfully.
- Pilot with one admin, one instructor, and two students using a small,
  unquestionably authorized set of MP4s.

## Explicitly deferred

- Email invitations and password recovery
- Per-group library membership
- Multiple organizations or gyms
- FFmpeg, `ffprobe`, transcoding, remuxing, server-side thumbnail extraction,
  and workers
- Multipart/resumable uploads and files larger than 5 GiB
- HLS/adaptive bitrate streaming
- Position and technique graphs
- Bookmarks and study queues
- AI transcription
- Native mobile/TV apps and offline downloads
- Payments, public registration, and public sharing
- Audit-event UI
- Terraform, Kubernetes, microservices, Redis, and message queues
- Complex CI/CD and zero-downtime deployment

Add a deferred feature only after the working MVP exposes a concrete problem it
solves.

## Manual tasks for the user

### Before Milestone 1

- Create the private Git repository and place this `AGENTS.md` at its root.
- Install Docker Desktop, Go, Node.js, and Git, or confirm Docker-only local
  development is preferred.
- Decide the bootstrap admin email.

### Before Milestone 2

- Create a DigitalOcean account/project and enable MFA, but do not create the
  Droplet yet.
- Create a private Spaces bucket in the region closest to the users.
- Create least-privilege Spaces credentials for this bucket.
- Put credentials in local environment variables, never in Git or prompts.
- Prepare one small H.264/AAC MP4 for testing.
- Confirm whether each test video is `self_created`, `licensed_for_group`, or a
  private `personal_purchase`.

### Before Milestone 4

- Buy or choose the domain/subdomain.
- Create the Droplet.
- Configure DNS and the DigitalOcean firewall using the deployment guide.
- Put production secrets directly on the server.
- Set DigitalOcean billing and resource alerts.
- Run and inspect a real database restore.

### Pilot

- Choose one instructor and two students.
- Upload only a few clearly authorized videos.
- Test on the phones and browsers people actually use.
- Track whether users resume videos and create/revisit notes for two weeks.
- Review DigitalOcean charges before importing the full library.

## Completion report

At the end of each milestone, report:

- What now works.
- Files and migrations changed.
- Tests and commands actually run.
- Anything unverified.
- Manual steps the user must perform.
- Deferred problems discovered.

Do not call a milestone complete until its gate passes. Do not begin the next
milestone automatically.
