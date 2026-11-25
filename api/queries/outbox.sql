-- Email outbox (ADR 0010).

-- name: EnqueueEmail :exec
INSERT INTO outbox_emails (id, kind, to_email, payload)
VALUES (@id, @kind, @to_email, @payload);

-- name: ClaimDueEmails :many
-- Leases up to 10 due rows for 5 minutes and counts the attempt. SKIP LOCKED lets several instances
-- claim at once without waiting for, or taking, each other's rows. A crash lets the lease expire.
UPDATE outbox_emails o
SET attempts = o.attempts + 1,
    next_attempt_at = now() + interval '5 minutes'
WHERE o.id IN (
    SELECT d.id FROM outbox_emails d
    WHERE d.status = 'pending' AND d.next_attempt_at <= now()
    ORDER BY d.next_attempt_at, d.id
    LIMIT 10
    FOR UPDATE SKIP LOCKED
)
RETURNING o.id, o.kind, o.to_email, o.payload, o.attempts;

-- name: MarkEmailSent :exec
-- The payload holds raw tokens: it does not outlive the send.
UPDATE outbox_emails
SET status = 'sent', sent_at = now(), payload = NULL, last_error = NULL
WHERE id = @id AND status = 'pending';

-- name: RescheduleEmail :exec
-- After attempt n the next one is due in 2^(n-1) minutes (1, 2, 4…); after the 8th the row fails.
UPDATE outbox_emails
SET status = CASE WHEN attempts >= 8 THEN 'failed' ELSE 'pending' END,
    next_attempt_at = now() + interval '1 minute' * power(2, greatest(attempts - 1, 0)),
    last_error = @last_error::text
WHERE id = @id AND status = 'pending';

-- name: ClearFailedEmailPayloads :execrows
UPDATE outbox_emails
SET payload = NULL
WHERE status = 'failed' AND payload IS NOT NULL AND created_at < @created_before;

-- name: PurgeSentEmails :execrows
DELETE FROM outbox_emails
WHERE status = 'sent' AND sent_at < @sent_before::timestamptz;
