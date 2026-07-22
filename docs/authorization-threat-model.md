# Libraries and authorization threat model

## Scope and trust boundaries

Milestone 3 protects user administration, personal and shared libraries, membership and instructor assignments, audit history, and the content-basis placement rule. The Go API and PostgreSQL are trusted enforcement points. Browsers, identifiers, roles, ownership claims, and library identifiers supplied by clients are untrusted.

The approved permission matrix in `authorization-permission-matrix.md` is normative. Every protected handler authenticates first and then calls the central authorization policy; handlers do not reproduce role logic.

## Threats and controls

| Threat | Control | Residual risk |
| --- | --- | --- |
| Horizontal resource enumeration | Policy-filtered lists and indistinguishable `404` responses for missing/inaccessible identifiers | Timing can still vary with database/cache behavior |
| Global-role bypass | Central policy requires role plus membership and ownership where applicable | Incorrectly constructed policy input remains a code-review concern |
| Personal-library disclosure | Personal libraries are owner-only; database prevents memberships on personal libraries | Database operators remain trusted |
| Instructor overreach | Shared-library upload policy requires active instructor assignment; mutation additionally requires resource ownership | Actual catalog/media handlers must call these policies in later milestones |
| Stale privilege after role/membership change | Policies read current state; role changes and disablement revoke sessions transactionally | Already-running requests may finish under their original transaction snapshot |
| Removal of final administrator | Row locking plus enabled-admin count protects the invariant transactionally | Direct database superusers can bypass application operations |
| Unauthorized content sharing | Central content-placement policy plus database trigger rejects `personal_purchase` in shared libraries | Future content tables must use the constrained content-record boundary |
| Audit tampering or secret leakage | Append-only trigger; bounded structured details; allowlisted event writers; no request bodies or capabilities | Database superusers remain capable of modification |
| CSRF on privileged mutations | Existing authenticated mutation path requires the session CSRF token before policy evaluation | Same-origin script execution bypasses CSRF |
| Confused-deputy administrative actions | Actor identity comes only from the validated session; actor and target are separate policy inputs and audit fields | Administrators retain their explicitly approved powers |

## Security invariants

- Every user has exactly one owner-only personal library.
- A personal library never has membership rows and never changes type or owner.
- Shared-library access requires an active membership, including for administrators reading content.
- Only administrators manage users, shared libraries, memberships, and audit history.
- Only globally eligible users receive instructor assignments.
- Disabling or changing a user role revokes all active sessions in the same transaction.
- The final enabled administrator cannot be disabled or demoted.
- Audit events are append-only and contain no authentication or storage capabilities.
- `personal_purchase` can only reference a personal library, enforced in policy and PostgreSQL.

## Deferred work

Catalog ownership, draft/publication state, media upload, and playback handlers do not exist in this milestone. The central policy exposes decisions for those future callers, and policy tests cover them, but this milestone does not create placeholder business endpoints.

