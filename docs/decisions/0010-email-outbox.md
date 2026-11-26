# 0010. Transactional outbox for emails

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
Invitations and password resets depend on email. employee-portal sent emails asynchronously after commit; if the process stopped before sending, the email was lost and nobody knew.

## Decision
- A use case that needs an email inserts a row into `outbox_emails` in the **same transaction** as the change that caused it.
- After commit, the API tries to send it right away.
- A worker goroutine retries unsent rows with backoff and records attempts and the last error.
- SMTP through `go-mail`; Mailpit locally.

### Implementation details (M1 design, 2026-11-25)
- **Table `outbox_emails`** (global, no row-level security, ADR 0008): `id`, `kind` (`password_reset`, `password_changed`, later `invitation`), `to_email`, `payload` (`jsonb`), `status` (`pending` | `sent` | `failed`), `attempts`, `next_attempt_at`, `sent_at`, `last_error` (truncated, no secrets), `created_at`.
- **Payloads are cleared once sent** (and on failed rows once their token has expired). A reset payload carries the raw token, which must not stay readable in the database; the row keeps kind, recipient and status for audit.
- **Claiming by lease:** one statement increments `attempts` and moves `next_attempt_at` 5 minutes ahead for up to 10 due rows picked with `FOR UPDATE SKIP LOCKED`, then commits. Sending happens outside any transaction; the row is then marked `sent` or rescheduled. Safe with several instances; a crash lets the lease expire and the email is retried (at least once, as above).
- **Backoff** 1, 2, 4… minutes, at most 8 attempts, then `failed`. Failing clears the payload. The claim also gives up (marks `failed`) on a due row whose attempts are used up, which happens when the process stopped during the last one, and on a `password_reset` older than its 1-hour token, so an expired link is never sent. On shutdown, claimed rows not yet tried are released: their attempt is undone and they are due again at once.
- **"Right away" means waking the worker** through a channel after commit, so there is one sending path; without a signal it polls every 10 seconds. `Message-ID` is the outbox ID. On shutdown the worker finishes the email in flight and stops.
- **Layers:** use cases enqueue with `repo.OutboxRepo` inside their unit of work; `repo.Mailer` (`repo/webapi/mail`, go-mail) renders embedded `html/template` and text templates and sends; the loop lives in `internal/app` and calls a `mail` use case.
- **SMTP:** `SMTP_TLS` is a mode, `none | starttls | tls` (production providers use STARTTLS on 587), with explicit connect and send timeouts. `SMTP_SEND_TIMEOUT` is at most 20 seconds, so a batch of 10 sent one by one stays within its 5-minute lease and a slow server cannot cause duplicates.
- **Daily maintenance** runs in the same loop: sessions 90 days after they ended (ADR 0006), `login_history` after 365 days (ADR 0013), sent outbox rows 30 days after sending and failed ones 30 days after they were created. Both keep the recipient (personal data), so neither stays forever; 30 days leaves time to look into a failure.

## Alternatives considered
- **Send inside the request:** a slow or failing SMTP server fails the request, or the email is sent for a change that later rolls back.
- **Send asynchronously after commit (as before):** lost on crash or restart.
- **A message broker:** reliable, but a new service to run for a handful of emails.

## Consequences
- An email is sent if and only if its change was committed, and it survives restarts.
- A crash between sending and marking as sent can send an email twice; the content is safe to repeat.
- One more table and a background worker in the API process.
