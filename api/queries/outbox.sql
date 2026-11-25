-- Email outbox (ADR 0010).

-- name: EnqueueEmail :exec
INSERT INTO outbox_emails (id, kind, to_email, payload)
VALUES (@id, @kind, @to_email, @payload);

-- name: ClaimDueEmails :many
-- Leases up to 10 due rows for 5 minutes and counts the attempt. SKIP LOCKED lets several instances
-- claim at once without waiting for, or taking, each other's rows. A crash lets the lease expire.
-- A due row is given up instead (status 'failed', payload cleared) when it has used its 8 attempts,
-- which happens when the process stopped during the last one, or when it is a password reset older
-- than its token (1 hour). The caller sends only the rows returned as 'pending'.
WITH due AS (
    SELECT d.id,
           (d.attempts >= 8 OR (d.kind = 'password_reset' AND d.created_at < now() - interval '1 hour')) AS give_up
    FROM outbox_emails d
    WHERE d.status = 'pending' AND d.next_attempt_at <= now()
    ORDER BY d.next_attempt_at, d.id
    LIMIT 10
    FOR UPDATE SKIP LOCKED
)
UPDATE outbox_emails o
SET attempts = CASE WHEN due.give_up THEN o.attempts ELSE o.attempts + 1 END,
    status = CASE WHEN due.give_up THEN 'failed' ELSE 'pending' END,
    payload = CASE WHEN due.give_up THEN NULL ELSE o.payload END,
    last_error = CASE WHEN due.give_up THEN coalesce(o.last_error, 'given up: no attempt left') ELSE o.last_error END,
    next_attempt_at = now() + interval '5 minutes'
FROM due
WHERE o.id = due.id
RETURNING o.id, o.kind, o.to_email, o.payload, o.attempts, o.status;

-- name: MarkEmailSent :exec
-- The payload holds raw tokens: it does not outlive the send.
UPDATE outbox_emails
SET status = 'sent', sent_at = now(), payload = NULL, last_error = NULL
WHERE id = @id AND status = 'pending';

-- name: RescheduleEmail :exec
-- After attempt n the next one is due in 2^(n-1) minutes (1, 2, 4…); after the 8th the row fails
-- and its payload is cleared (a reset token has expired by then).
UPDATE outbox_emails
SET status = CASE WHEN attempts >= 8 THEN 'failed' ELSE 'pending' END,
    payload = CASE WHEN attempts >= 8 THEN NULL ELSE payload END,
    next_attempt_at = now() + interval '1 minute' * power(2, greatest(attempts - 1, 0)),
    last_error = @last_error::text
WHERE id = @id AND status = 'pending';

-- name: ReleaseEmails :exec
-- Returns claimed rows that were never tried (shutdown): the attempt is undone and they are due now.
UPDATE outbox_emails
SET attempts = greatest(attempts - 1, 0), next_attempt_at = now()
WHERE id = ANY(@ids::uuid[]) AND status = 'pending';

-- name: ClearFailedEmailPayloads :execrows
-- A safety net: failing clears the payload already.
UPDATE outbox_emails
SET payload = NULL
WHERE status = 'failed' AND payload IS NOT NULL AND created_at < @created_before;

-- name: PurgeSentEmails :execrows
DELETE FROM outbox_emails
WHERE status = 'sent' AND sent_at < @sent_before::timestamptz;
