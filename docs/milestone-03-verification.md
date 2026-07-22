# Historical Milestone 3 verification report

This report describes the older library-authorization milestone before the simplified MVP refactor. Its worker and invitation references are historical, not current operating instructions. See `README.md` and `AGENTS.md` for the current application scope.

Date: 2026-07-21

## Automated results

- Go formatting, vet, and race-enabled tests pass.
- React lint, component tests, and production build pass.
- Docker Compose configuration validates.
- A clean disposable PostgreSQL volume migrates through version 3 with `dirty = false`.
- All API, worker, migration, and web images build and the health/readiness endpoints pass.
- Central policy tests cover role, membership, ownership, library type/state, draft visibility, upload eligibility, and all content bases.
- PostgreSQL integration tests cover automatic personal libraries, personal-library isolation, membership restrictions, instructor eligibility, immediate membership removal, final-admin protection, transactional session revocation, assignment downgrade, append-only audit events, and the content-basis assertion.
- HTTP integration tests use separate admin, instructor, and student sessions to prove inaccessible IDs return `404`, administrators cannot inspect another personal library, students cannot create libraries or receive instructor assignment, unassigned instructors cannot enumerate a shared library, and membership removal takes effect immediately.

## Manual review status

- Permission matrix: approved by the user on 2026-07-21.
- Real uploads remain impossible because upload endpoints are Milestone 5.
- Before Milestone 5 or real media use, the user must repeat the manual role/ID tampering checks from `AGENTS.md` with separate browser accounts and inspect cookies/network traffic. Draft-title, thumbnail, media-URL, and signed-URL checks cannot occur until those resources exist.

## Deliberately deferred

- Catalog resources and concrete ownership transfer endpoints: Milestone 4.
- Upload authorization callers and database content rows: Milestone 5.
- Signed playback URLs: Milestone 7.
- Production audit retention/export and deployment hardening: Milestone 9.
