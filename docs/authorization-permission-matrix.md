# Milestone 3 permission matrix

Status: **approved on 2026-07-21**

## Policy inputs

Every decision is made by a central backend policy from all applicable inputs:

```text
authenticated user + global role + account state
+ library type + library membership/assignment
+ resource owner + resource state + content basis
```

Frontend visibility is never an authorization control. A denied operation returns `404` whenever `403` would confirm that a private library, membership, user-scoped resource, draft, or other inaccessible resource exists. `401` is reserved for missing or invalid authentication. Administrative list endpoints return only resources the policy permits the caller to enumerate.

## Definitions

- Global roles are `admin`, `instructor`, and `student`.
- Library types are `personal` and `shared`.
- Every user owns exactly one personal library, created transactionally with the user. Personal libraries cannot accept memberships, be reassigned, or become shared.
- Shared-library membership levels are `student` and `instructor`. An instructor assignment is an active `instructor` membership, not a global-role shortcut.
- A global instructor may be a student member of a shared library; that membership grants viewing, not uploading.
- Resource owner means the user recorded as owner of the future catalog/media resource. Ownership changes are admin-only and audited.
- Disabled accounts have no access regardless of any other input.
- “Published content” and content mutation policies are defined now for use by later milestones; Milestone 3 does not add catalog or upload endpoints.

## User and role administration

| Operation | Admin | Instructor | Student | Denial |
| --- | --- | --- | --- | --- |
| List users | Allow | Deny | Deny | `404` route/resource concealment |
| View a user’s administrative profile | Allow | Self only through current-session data | Self only through current-session data | `404` |
| Change global role | Allow, except removing the final enabled admin | Deny | Deny | `404` |
| Disable or re-enable user | Allow, except disabling the final enabled admin | Deny | Deny | `404` |
| Revoke another user’s sessions | Allow | Deny | Deny | `404` |

Changing a role, disabling a user, or changing a resource owner revokes that affected user’s active sessions in the same transaction. Administrators cannot change their own role or disable themselves when that would leave no enabled administrator.

## Library lifecycle and enumeration

| Operation | Admin | Instructor | Student | Denial |
| --- | --- | --- | --- | --- |
| List personal libraries | Own only | Own only | Own only | Omit inaccessible rows |
| View personal-library metadata/content | Own only | Own only | Own only | `404` |
| Modify personal-library metadata | Own display name only | Own display name only | Own display name only | `404` |
| Delete/archive personal library | Deny | Deny | Deny | `404` |
| Create shared library | Allow | Deny | Deny | `404` |
| List/view shared library | If active member | If active member | If active member | Omit/`404` |
| Edit/archive shared library | Allow | Deny | Deny | `404` |
| Delete shared library permanently | Deny | Deny | Deny | `404` |

Global administrators do **not** implicitly read personal libraries. They also need active membership to enumerate or read shared-library content. An administrator may add themselves to a shared library through the audited membership-management operation.

## Shared-library membership and instructor assignment

| Operation | Admin | Instructor | Student | Denial |
| --- | --- | --- | --- | --- |
| List members of shared library | Allow | Own membership only | Own membership only | `404`/filtered |
| Add or remove student membership | Allow | Deny | Deny | `404` |
| Add or remove instructor assignment | Allow, target must have global `instructor` or `admin` role | Deny | Deny | `404` |
| Change membership level | Allow, subject to role compatibility | Deny | Deny | `404` |
| Add membership to personal library | Deny | Deny | Deny | `404` |

An archived shared library retains memberships for referential integrity but grants no new mutation or upload operations. Removing a membership takes effect on the next request because policies read current database state.

## Content operations for later catalog/media milestones

| Operation | Admin | Instructor | Student |
| --- | --- | --- | --- |
| View published content in personal library | Own library | Own library | Own library |
| View published content in shared library | Active membership required | Active membership required | Active membership required |
| View draft content in personal library | Own resources | Own resources | Deny |
| View draft content in shared library | Member and owner, or member admin | Instructor member and owner | Deny |
| Upload/create in personal library | Own library | Own library | Deny |
| Upload/create in shared library | Active membership required | Active instructor membership required | Deny |
| Edit/publish/archive owned content | Active permitted library access | Active instructor assignment and ownership | Deny |
| Edit/publish/archive another user’s content | Active membership and admin role | Deny unless ownership was explicitly transferred | Deny |
| Change content owner | Active membership and admin role; new owner must have permitted library access | Deny | Deny |

“Upload/create” here is policy behavior only; actual catalog and media operations are later milestones. Whether instructor publication additionally requires admin approval remains a separate pre-catalog product decision and does not weaken the ownership or membership checks above.

## Content-basis invariant

Allowed content bases are exactly:

- `self_created`
- `licensed_for_group`
- `personal_purchase`

| Content basis | Personal library | Shared library |
| --- | --- | --- |
| `self_created` | Allow | Allow |
| `licensed_for_group` | Allow | Allow |
| `personal_purchase` | Allow | **Deny** |

The backend central policy rejects invalid placement before persistence. The database also enforces the invariant on every table that eventually combines a library reference with content basis. Moving or copying `personal_purchase` content to a shared library is denied. Changing a personal library into a shared library is structurally impossible.

## Audit requirements

The following successful actions create an append-only audit event in the same database transaction as the action:

- global role changes;
- user disable/re-enable and administrative session revocation;
- shared-library create/edit/archive;
- membership add/remove/level change, including an administrator adding themselves;
- resource ownership transfer;
- future publish/archive operations and content-basis changes.

Audit events record actor, action, target type and identifier, request ID, timestamp, and a bounded JSON object containing non-secret before/after facts. They never contain passwords, session or CSRF tokens, invitation tokens, signed URLs, storage credentials, or unrestricted request bodies. Audit events cannot be updated or deleted by application code.

## Mandatory denial tests

Implementation is not accepted without tests proving at least:

1. A student cannot create, edit, archive, upload to, or manage membership for any library.
2. An instructor cannot enumerate or access an unassigned shared library.
3. A global instructor with only student membership cannot upload.
4. An instructor cannot edit another instructor’s resource without an explicit ownership transfer.
5. No user or administrator can access another user’s personal library.
6. Non-admin users cannot enumerate global users or shared-library membership.
7. Membership removal takes effect on the next request.
8. Role change and disablement revoke existing sessions immediately.
9. A final enabled administrator cannot be demoted or disabled.
10. `personal_purchase` cannot be persisted in, moved to, or copied to a shared library.
11. Invalid IDs and inaccessible IDs produce indistinguishable `404` responses where existence is sensitive.
12. Every privileged mutation emits the required non-secret audit event atomically.
