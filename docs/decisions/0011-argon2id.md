# 0011. argon2id for password hashing

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
employee-portal used bcrypt, which truncates passwords at 72 bytes. The new repository starts without existing hashes, so there is nothing to migrate.

## Decision
Hash passwords with argon2id (`golang.org/x/crypto/argon2`) using OWASP's recommended parameters, stored in the standard encoded format with parameters and salt. Passwords are 8 to 128 characters. Failed lookups still run a dummy hash so response time does not reveal whether an email exists.

### Implementation details (M1 design, 2026-11-25)
- Parameters: OWASP's minimum `m=19 MiB, t=2, p=1`, 16-byte salt, 32-byte key, PHC string format (`$argon2id$v=19$m=19456,t=2,p=1$…`). A successful sign-in rehashes when the stored parameters are older than the current ones.
- At most `GOMAXPROCS` hashes run at once (a semaphore that gives up when the request is cancelled). Without it, a burst of sign-in attempts multiplies 19 MiB per attempt and can exhaust a small server's memory.
- Comparison is constant-time. Length is 8 to 128 characters with no composition rules (NIST SP 800-63B). Checking passwords against known breaches is in the backlog (it needs an external service).
- The implementation lives in `pkg/password`; use cases depend on a `PasswordHasher` interface in `repo/contracts.go`, where the template puts every non-domain dependency of a use case.
- Hashing never runs inside a database transaction: it would hold a pooled connection for the whole hash.

## Alternatives considered
- **bcrypt:** still acceptable, but has the 72-byte limit.
- **scrypt:** fine, less commonly recommended first.

## Consequences
- Memory-hard hashing raises the cost of offline attacks.
- Each login uses tens of megabytes briefly; the parameters are sized for a small server and can be raised later, since the encoded hash records them.
