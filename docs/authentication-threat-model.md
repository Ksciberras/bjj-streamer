# Authentication threat model

## Scope and assets

Authentication protects account credentials, authenticated sessions, and administrative account management. There is no public registration or active invitation endpoint. An administrator creates accounts for known users and communicates initial passwords outside the application.

The browser is untrusted. PostgreSQL and the API are trusted components on the private application network. TLS termination is required in production.

## Threats and controls

| Threat | Control | Residual risk |
| --- | --- | --- |
| Public account creation | Account creation is available only to an authenticated administrator with a valid CSRF token | Administrators are trusted to create only known users |
| Credential database theft | Argon2id with a random salt per password; passwords are never logged or returned | Weak user-chosen passwords remain guessable at the configured Argon2 cost |
| Session database theft | 256-bit random session tokens; only SHA-256 hashes are stored; session cookie is HTTP-only | An active browser cookie can still be stolen by endpoint compromise |
| Session fixation | Login creates a new session and revokes any presented prior session | Concurrent sessions remain allowed intentionally |
| CSRF | Strict SameSite session cookie plus a per-session CSRF token required for authenticated mutations | Same-site script injection bypasses CSRF; XSS prevention remains essential |
| Brute force | Per-IP and per-identity in-process login limits plus uniform invalid-credential responses | Limits reset at restart; acceptable only for one initial Droplet |
| Disabled or stale access | Session state, expiry, revocation, and user disabled state are checked in PostgreSQL on every authenticated request | Database availability is required |
| Password-reset persistence | Resetting a password revokes every active session for the target user in the same transaction | An administrator can intentionally take over an account |
| Privilege-change persistence | Role and disabled-state changes revoke active sessions transactionally | Administrators remain trusted |
| User enumeration | Login returns a generic failure and uses a dummy Argon2 hash for missing accounts | Administrative account pages intentionally reveal users to administrators |
| Secret leakage through logs | Handlers do not log request bodies, cookies, passwords, session tokens, or query strings | Infrastructure logging must follow the same restriction |
| Bootstrap takeover | Interactive bootstrap succeeds only while the users table is empty, protected by a serializable transaction and advisory lock | Host/database administrators remain trusted |

## Security invariants

- Passwords and raw session and CSRF tokens are never persisted or logged.
- Disabled users and revoked, expired, or idle sessions fail on their next request.
- Cookie-authenticated state changes require a valid session and CSRF token.
- Account creation, password resets, role changes, and disable operations are administrator-only.
- Password resets and authorization changes revoke target-user sessions atomically.
- The final enabled administrator cannot be disabled or demoted.
- Production refuses to issue authentication cookies without Secure enabled.

## Legacy compatibility

Migration `000002` created an `invitations` table. The table and any stored rows are preserved because applied migrations and stored data are immutable during this refactor. No HTTP route consumes or creates invitations now.

## Deferred risks

MFA, email delivery, password-recovery email, distributed rate limiting, and organization management are outside the MVP.
