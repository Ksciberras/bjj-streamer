# BJJ Streaming Platform — Agent Instructions

## Mission

Build an invite-only, cloud-hosted BJJ instructional platform for a small group
of friends and training partners. The product is a learning system, not merely
a video gallery: it must combine secure streaming with technique organization,
timestamped notes, bookmarks, progress, and study queues.

The expected initial scale is 5–20 users and approximately 1 TiB or less of
media. Optimize for correctness, privacy, recoverability, and a simple operating
model. Do not optimize prematurely for large-scale public traffic.

## Fixed product decisions

- Hosting: DigitalOcean; the user cannot self-host.
- Compute: one Droplet initially.
- Media: a private DigitalOcean Spaces bucket.
- Application: React + TypeScript frontend, Go backend, PostgreSQL.
- Deployment: Docker Compose behind Caddy.
- Processing: a Go worker invoking `ffprobe` and FFmpeg.
- Registration: invite-only. There is no public sign-up.
- Roles: `admin`, `instructor`, and `student`.
- Uploading: only admins and instructors may upload.
- Libraries: every user has a private personal library; authorized groups can
  also access licensed shared libraries.
- Initial playback: browser-compatible MP4 using HTTP range requests and
  short-lived signed object-storage URLs.
- HLS, native applications, offline downloads, AI transcription, payments, and
  public sharing are post-MVP features.

Do not replace these decisions without recording the reason in an architecture
decision record and obtaining user approval.

## Legal and content constraints

The platform must not facilitate copyright infringement or DRM circumvention.
Every media asset must declare one of these content bases:

- `self_created`
- `licensed_for_group`
- `personal_purchase`

Enforce this invariant in backend code and, where practical, the database:

```text
personal_purchase -> personal library only
```

A checkbox or frontend warning is not enforcement. Reject invalid sharing on
the server. Do not implement download-site scraping, DRM removal, credential
sharing, or mechanisms intended to conceal unauthorized redistribution.

## Roles and authorization

Authorization is determined by all three factors:

```text
global role + library membership + resource ownership
```

### Admin

- Invite, disable, and manage users.
- Assign global roles.
- Create shared libraries and manage their memberships.
- Upload, edit, publish, archive, and administer permitted content.
- View audit history.

### Instructor

- Upload only to shared libraries to which they are assigned as an instructor,
  or to their own personal library.
- Edit, publish, and archive their own content within those libraries.
- Cannot edit another instructor's content unless explicitly made its owner.
- Cannot manage global users, roles, or unrelated libraries.

### Student

- View published content in assigned libraries.
- Create private notes, bookmarks, playback progress, and study queues.
- Cannot upload, publish, edit catalog data, or view drafts.

Frontend visibility is not security. Every protected backend operation must
call a central authorization policy. Prefer `404` over `403` when revealing a
resource's existence would disclose private information.

## Architecture

Use a modular monolith:

```text
React web app -> Go API -> PostgreSQL
      |             |
      +-------> private Spaces bucket
                         ^
                         |
                   Go/FFmpeg worker
```

Large media bytes must not pass through the Go API. The browser uploads
directly to Spaces using short-lived presigned multipart requests. Playback is
authorized by the API and delivered from Spaces using short-lived signed GET
URLs.

Suggested repository structure:

```text
apps/
  web/
cmd/
  api/
  worker/
internal/
  auth/
  authorization/
  catalog/
  learning/
  libraries/
  media/
  users/
db/
  migrations/
deploy/
docs/
```

Do not introduce Kubernetes, microservices, Redis, a message broker, or a
managed database unless measured requirements justify them and the user
approves the operational cost.

## Domain model

Identity and access:

```text
users
sessions
invitations
libraries
library_members
audit_events
```

Catalog:

```text
instructors
instructionals
volumes
chapters
positions
techniques
technique_aliases
chapter_positions
chapter_techniques
```

Media:

```text
media_assets
multipart_uploads
processing_jobs
```

Personal learning data:

```text
playback_progress
notes
bookmarks
study_queue_items
```

All learning records are scoped by `user_id`. Private is the default. Preserve
learning records if a media rendition is replaced or catalog content is
archived.

## Media lifecycle

Uploads follow:

```text
created -> uploading -> uploaded -> inspecting
                 |                      |
                 +-> cancelled          +-> ready
                 +-> expired            +-> processing -> ready
                                        +-> failed
```

Publication follows:

```text
draft -> processing -> ready -> published -> archived
```

The worker should make this decision:

```text
H.264 + AAC + MP4                 -> retain for direct playback
compatible codecs, wrong container -> remux without re-encoding
incompatible codecs                -> transcode to H.264/AAC MP4
DRM-protected or unreadable        -> reject with a clear failure
```

Originals must never be overwritten. Worker jobs must be idempotent, retryable,
and protected by expiring leases. Run one expensive FFmpeg job at a time on the
initial Droplet. Remove temporary files after both success and failure.

## Engineering rules

- Work on one milestone at a time.
- Inspect the repository and existing instructions before editing.
- Do not implement future milestones speculatively.
- Keep authorization in explicit, testable backend policies.
- Validate state transitions centrally; handlers must not update status fields
  arbitrarily.
- Use database constraints for invariants that the database can enforce.
- Use migrations for every schema change; never edit an applied migration.
- Generate object keys on the server. Never accept arbitrary storage paths from
  clients.
- Never expose Spaces credentials, password hashes, session tokens, presigned
  URL query strings, or secrets in logs.
- Use secure HTTP-only cookies for sessions. Rotate session IDs on login and
  privilege changes, and support immediate revocation.
- Require CSRF protection for cookie-authenticated state-changing requests.
- Use Argon2id for passwords and rate-limit login and invitation endpoints.
- Use structured logs with request and correlation IDs.
- Keep production PostgreSQL and worker control endpoints off the public
  internet.
- Prefer archiving over destructive deletion. Permanent deletion needs an
  explicit retention policy and auditable operation.

## Verification rules

Every milestone must include implementation, migrations where relevant, tests,
documentation, and a verification report. Run all applicable commands before
claiming completion, typically:

```bash
go fmt ./...
go vet ./...
go test ./...
npm run lint
npm test
npm run build
docker compose config
```

Do not report a command as passing if it was not run. If environment limitations
prevent verification, report exactly what remains unverified.

Prioritize tests for:

- Authorization policies and resource enumeration.
- Session rotation, expiry, revocation, and CSRF.
- Content-basis sharing restrictions.
- Upload state transitions, retries, and cleanup.
- Worker leases, crashes, and idempotency.
- Signed playback authorization and expiry.
- Per-user isolation of notes, bookmarks, queues, and progress.

## Milestone execution protocol

For each milestone:

1. Start from the latest accepted main branch.
2. Create a focused branch such as `milestone/03-authorization`.
3. Restate the milestone scope and inspect relevant existing code.
4. Write or update the design notes and threat model before security-sensitive
   implementation.
5. Implement the smallest complete slice.
6. Add unit, integration, and end-to-end tests proportional to risk.
7. Run formatting, linting, tests, builds, and configuration validation.
8. Summarize changed files, decisions, verification results, and remaining
   risks.
9. Stop. Do not begin the next milestone automatically.

One focused pull request and commit series per milestone is preferred. Never
hide unrelated cleanup inside a milestone.

## Milestones and gates

### 1. Foundation

Implement the Go API and worker entry points, React application, PostgreSQL,
migrations, Docker Compose, configuration validation, structured logging,
health/readiness endpoints, graceful shutdown, tests, CI, and development
documentation.

Gate: a clean checkout starts with documented commands, a clean database
migrates successfully, and backend/frontend builds and tests pass. Do not add
placeholder business endpoints.

### 2. Invite-only authentication

Document the authentication threat model. Implement first-admin bootstrap,
expiring single-use invitations, Argon2id passwords, secure server-side
sessions, login/logout, rotation, revocation, rate limiting, CSRF protection,
and a minimal authentication UI.

Gate: public signup is impossible; invalid, expired, and consumed invitations
fail; revoked sessions stop immediately; secrets never appear in logs.

### 3. Libraries and authorization

Write the permission matrix first. Implement automatic personal libraries,
shared libraries, membership administration, instructor assignments, central
authorization policies, audit events, and content-basis enforcement.

Gate: students cannot upload; instructors cannot access unassigned libraries;
users cannot enumerate inaccessible resources; personal purchases cannot be
shared. Exhaustive policy and integration tests are mandatory. Do not proceed
until this gate is reviewed manually.

### 4. Instructional catalog

Implement instructors, instructionals, volumes, chapters, hierarchical
positions, techniques, aliases, ordering, draft/published/archived states,
search, filters, instructor editing, and student browsing.

Gate: instructors can edit only owned content in assigned libraries; students
cannot see drafts; search respects membership; archive operations preserve
learning references.

### 5. Direct multipart uploads

Implement an S3-compatible storage boundary, local object-storage development
support, multipart initiation, short-lived part signing, resume, completion,
cancellation, validation, progress, and stale-upload cleanup.

Gate: a realistically large upload travels directly from browser to object
storage, survives interruption, and cannot be initiated by a student or an
unassigned instructor. Storage credentials never reach React.

### 6. Inspection and processing

Implement durable processing jobs, leases, retries, `ffprobe` inspection,
compatibility classification, remuxing, H.264/AAC transcoding, thumbnails,
progress, failure reporting, and temporary-file cleanup.

Gate: originals are preserved; jobs are idempotent; worker crashes recover;
failed jobs can be retried; publishing is impossible before media is ready.

### 7. Secure playback

Implement playback authorization, short-lived signed GET URLs, MP4 range
playback, responsive player controls, per-user progress, resume, and completion.

Gate: unauthorized users cannot obtain URLs; drafts are unavailable to
students; seeking works without a full download; Safari and Chromium playback
are tested; progress is isolated per user.

### 8. Learning workflow

Implement timestamped private notes, click-to-seek, bookmarks, personal study
queues, continue watching, recent activity, completion, position/technique
browsing, and cross-instructional search.

Gate: learning data is private; timestamps are within media duration; two users
have independent state; search exposes only authorized published content.

### 9. Production deployment and recovery

Implement production containers and Compose, Caddy/HTTPS, non-root operation,
secret injection, backups, retention, tested restoration, monitoring, alerts,
deployment, rollback, security headers, and runbooks.

Gate: PostgreSQL and Spaces are private; only intended ports are reachable; a
fresh Droplet can be rebuilt; a backup has actually been restored; no production
secret exists in Git or container images.

### 10. Controlled pilot

Pilot with one admin, one instructor, two students, and a small set of
self-created or properly licensed media. Exercise invitations, upload,
processing, publication, playback, private notes, revocation, worker failure,
backup, and restore.

Gate: the pilot runs for two weeks without a permission leak; failures and
costs are understood; users actually return to the study features. Only pilot
evidence should determine whether the next feature is transcription, HLS,
casting, or something else.

## Explicit MVP exclusions

- Public registration
- Payments and subscriptions
- Native mobile or TV applications
- Offline downloads
- AI transcription
- HLS/adaptive streaming
- Livestreaming
- Public share links
- Social feeds and comments
- Automated media scraping
- DRM circumvention
- Kubernetes and microservices

## Tasks requiring the user to perform or approve manually

Agents must not pretend these tasks were completed merely because code or
documentation was generated.

### Before development

- Create the Git repository and choose its visibility.
- Confirm the initial DigitalOcean region based on the users' location.
- Estimate current media volume, largest file size, and expected monthly viewing
  hours; these numbers influence storage and transfer limits.
- Decide whether instructors may self-publish or require admin approval.
- Decide the retention period before archived media can be permanently deleted.
- Confirm that every shared media item is self-created or licensed for the
  intended group. Keep licence records outside or alongside the platform.

### DigitalOcean setup

- Create the DigitalOcean account/project and enable MFA.
- Create the private Spaces bucket in the chosen region.
- Create least-privilege Spaces access credentials for the application.
- Keep root/account recovery credentials outside the application.
- Create the Droplet only when the production milestone is ready.
- Configure DigitalOcean firewall rules and DNS.
- Set billing alerts and resource alerts.
- Review current storage and outbound-transfer pricing before importing the full
  library.

Never paste long-lived production secrets into Codex prompts, issue trackers,
commits, or chat. Put them directly into the approved secret store or production
environment.

### Manual security review

- Review the Milestone 3 permission matrix and denial tests before allowing real
  uploads.
- Create separate test users for admin, instructor, and student roles.
- Attempt horizontal privilege escalation by changing IDs in requests.
- Verify that an instructor cannot upload to an unassigned library.
- Verify that students cannot see draft titles, thumbnails, or media URLs.
- Inspect browser cookies and network requests for exposed secrets.
- Confirm that signed URLs expire and that the Spaces bucket has no public
  listing or anonymous object access.
- Rotate credentials immediately if they appear in Git, logs, or prompts.

### Deployment and recovery

- Purchase or configure the domain and DNS.
- Supply production secrets directly in the deployment environment.
- Apply firewall settings and verify them from an external network.
- Enable and review monitoring alerts.
- Download or provision the documented backup credentials.
- Perform a real restore into a clean database and inspect representative data.
- Test account recovery and admin session revocation.
- Keep an independent copy of the recovery runbook.

### Pilot operation

- Choose the pilot instructor and two students.
- Upload only a small, unquestionably authorized test library first.
- Test desktop and mobile playback on the devices the group actually uses.
- Collect concrete failures rather than feature requests alone.
- Review weekly resource use and DigitalOcean charges.
- Decide whether the platform is valuable based on repeat study behavior, not
  merely successful video playback.

## Completion reporting

At the end of each task, agents should report:

- Outcome first.
- Files and migrations changed.
- Tests and commands actually run, with results.
- Security or data-migration implications.
- Manual steps still required from the user.
- Any blocked or deliberately deferred work.

Do not call a milestone complete while its acceptance gate or required manual
verification is outstanding.
