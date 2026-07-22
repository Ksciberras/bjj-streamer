# Simplified MVP Milestone 3 verification

Date: 2026-07-22

## Implemented

- Authorized short-lived signed GET URLs for ready videos.
- Native responsive `<video>` player using direct object-storage delivery.
- Per-user playback progress with resume, periodic saving, pause saving, and navigation saving.
- Per-user timestamped notes with create, edit, delete, and click-to-seek.
- Mobile layouts for the player and note editor.

## Automated verification

- Shared video playback is authorized and private video playback is concealed from students.
- Real MinIO signed GET requests support byte ranges with `206 Partial Content` and `Accept-Ranges: bytes`.
- Two users watching the same video retain independent playback positions.
- Notes from one user are absent from another user's list and cannot be edited or deleted by ID.
- React component coverage opens the native player and seeks to a note timestamp.
- Go formatting, vet, race-enabled tests, React lint/tests/build, Docker Compose, migrations, and health checks pass.

## Manual gate checks still required

- In current Safari and Chromium, play and seek within a real H.264/AAC MP4.
- Leave the player, return, and confirm playback resumes near the saved position.
- Add, edit, delete, and select a note in both browsers.
- With two real user accounts, confirm independent progress and notes.

Deployment remains Milestone 4 and is deliberately absent here.
