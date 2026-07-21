# Authentication threat model

## Scope and assets

Milestone 2 protects account credentials, invitation capabilities, authenticated sessions, and administrative invitation creation. It does not grant library or content access; resource authorization is Milestone 3.

The browser is untrusted. PostgreSQL and the API are trusted components on the private application network. TLS termination is required in production. Email delivery is outside this milestone: an administrator must transfer the one-time invitation URL through a trusted channel.

## Threats and controls

| Threat | Control | Residual risk |
| --- | --- | --- |
| Public account creation | No signup route; account creation requires a random, stored-as-hash invitation token | Anyone possessing an unconsumed token may use it |
| Credential database theft | Argon2id with per-password random salt; passwords never logged or returned | Weak user-chosen passwords remain guessable at Argon2 cost |
| Session database theft | 256-bit random session tokens; only SHA-256 hashes stored; HTTP-only cookie | An active browser cookie can still be stolen by endpoint compromise |
| Session fixation | A new session identifier is created after every login; any presented prior session is revoked | Concurrent login sessions remain allowed intentionally |
| CSRF | Strict SameSite session cookie plus per-session random CSRF token required in a header for authenticated mutations | Same-site script injection bypasses CSRF; XSS prevention remains essential |
| Brute force and token probing | Per-IP and per-identity in-memory rate limits; uniform invalid-credential response | Limits reset on process restart; acceptable for one initial Droplet |
| Invitation replay | Transactional row lock and consumed timestamp; token is stored only as a hash | Administrator must transfer the URL securely |
| Expired/revoked access | Expiry and revocation are checked from PostgreSQL on every authenticated request | Database availability is required for every authenticated request |
| User enumeration | Login and invitation acceptance return generic failures; normalized email is not disclosed | Timing differences are reduced with a dummy password hash |
| Secret leakage through logs | Handlers never log bodies, cookies, passwords, invitation tokens, or query strings | Infrastructure access logs must retain the same restriction |
| Bootstrap takeover | Interactive command succeeds only while the users table is empty and uses a serializable transaction/advisory lock | Host/database administrators remain trusted |

## Security invariants

- Passwords, raw session tokens, raw CSRF tokens, and raw invitation tokens are never persisted or logged.
- Disabled users and revoked, expired, or idle sessions fail immediately on their next request.
- Invitation consumption and user creation occur in one transaction.
- Cookie-authenticated state changes require both a valid session and CSRF token.
- Authentication errors do not reveal whether an email, invitation, or session exists.
- Production refuses to issue authentication cookies without the secure-cookie configuration.

## Deferred risks

Distributed rate limiting, MFA, account recovery, email delivery, global user/role management, privilege-change session revocation, and authorization for domain resources are outside Milestone 2. Privilege-change revocation must be added with role management in Milestone 3.

