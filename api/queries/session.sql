-- Server-side sessions (ADR 0006). Times come from the API's clock (@now), so one clock decides
-- creation, expiry and refresh.

-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, token_hash, created_at, last_seen_at, idle_expires_at, absolute_expires_at, ip, user_agent)
VALUES (@id, @user_id, @token_hash, @created_at, @created_at, @idle_expires_at, @absolute_expires_at,
        sqlc.narg('ip'), sqlc.narg('user_agent'));

-- name: GetActiveSessionByTokenHash :one
-- One indexed lookup per request: the session, if not revoked and neither expiry reached, with its
-- user. The password hash is left out on purpose: it has no business in a request context.
SELECT s.id, s.user_id, s.created_at, s.last_seen_at, s.idle_expires_at, s.absolute_expires_at, s.ip, s.user_agent,
       u.email, u.full_name, u.phone, u.created_at AS user_created_at, u.updated_at AS user_updated_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash
  AND s.revoked_at IS NULL
  AND s.idle_expires_at > @now
  AND s.absolute_expires_at > @now;

-- name: TouchSession :execrows
-- Extends the idle expiry of an active session, capped at the absolute one. The last_seen_at
-- condition makes concurrent requests write at most once per refresh interval.
UPDATE sessions
SET last_seen_at = @now,
    idle_expires_at = least(@idle_expires_at::timestamptz, absolute_expires_at)
WHERE id = @id
  AND revoked_at IS NULL
  AND idle_expires_at > @now
  AND absolute_expires_at > @now
  AND last_seen_at < @refresh_before::timestamptz;

-- name: RevokeSession :execrows
-- Only an active session of that user: an ended one keeps the time it ended (retention).
UPDATE sessions
SET revoked_at = @now::timestamptz, revoked_reason = @reason::text
WHERE id = @id
  AND user_id = @user_id
  AND revoked_at IS NULL
  AND idle_expires_at > @now
  AND absolute_expires_at > @now;

-- name: RevokeOtherSessions :execrows
UPDATE sessions
SET revoked_at = @now::timestamptz, revoked_reason = @reason::text
WHERE user_id = @user_id
  AND id <> @except_id
  AND revoked_at IS NULL
  AND idle_expires_at > @now
  AND absolute_expires_at > @now;

-- name: PurgeEndedSessions :execrows
-- A session ended at the first of its revocation and its two expiries (least ignores NULL).
DELETE FROM sessions
WHERE least(revoked_at, idle_expires_at, absolute_expires_at) < @ended_before::timestamptz;
