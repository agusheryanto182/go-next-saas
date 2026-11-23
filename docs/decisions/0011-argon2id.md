# 0011. argon2id for password hashing

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
employee-portal used bcrypt, which truncates passwords at 72 bytes. The new repository starts without existing hashes, so there is nothing to migrate.

## Decision
Hash passwords with argon2id (`golang.org/x/crypto/argon2`) using OWASP's recommended parameters, stored in the standard encoded format with parameters and salt. Passwords are 8 to 128 characters. Failed lookups still run a dummy hash so response time does not reveal whether an email exists.

## Alternatives considered
- **bcrypt:** still acceptable, but has the 72-byte limit.
- **scrypt:** fine, less commonly recommended first.

## Consequences
- Memory-hard hashing raises the cost of offline attacks.
- Each login uses tens of megabytes briefly; the parameters are sized for a small server and can be raised later, since the encoded hash records them.
