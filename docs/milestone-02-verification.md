# Simplified MVP Milestone 2 verification

Date: 2026-07-22

## Implemented

- Additive `videos` schema with role-independent shared/private visibility, content-basis and size constraints, pending/ready/archived status, and server-generated object keys.
- Authenticated catalog browsing and basic search across title, instructor, instructional name, and tags.
- Admin/instructor single-presigned-PUT uploads of `.mp4`/`video/mp4` files up to 5 GiB.
- Direct browser-to-object-storage upload progress and API completion notification.
- Object existence, declared size, and MIME verification before a video becomes ready.
- Admin management of every video and instructor management of uploader-owned videos only.
- Shared ready videos for all authenticated users; private ready videos only for uploader and administrators.
- Local private MinIO bucket and DigitalOcean Spaces-compatible configuration.

## Automated verification

- Go formatting and vet pass for `cmd` and `internal` packages.
- Race-enabled Go unit and PostgreSQL integration tests pass.
- A real presigned PUT to MinIO, object HEAD, API completion, and catalog search pass.
- MinIO browser CORS preflight for the local web origin passes.
- React lint, component tests, and production build pass.
- Docker Compose validates, builds, migrates to version 4 cleanly, initializes the bucket, and reports healthy.

## Manual gate checks still required

- Upload a small, genuinely browser-compatible H.264/AAC MP4 through the browser as an administrator and instructor.
- Confirm a student sees shared ready videos and cannot see private videos or upload controls.
- Confirm an instructor cannot edit another instructor's video by using separate browser accounts.
- Confirm DigitalOcean Spaces bucket privacy and CORS when deployment credentials are supplied.

Playback is Milestone 3 and is deliberately absent here.
