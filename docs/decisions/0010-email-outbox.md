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

## Alternatives considered
- **Send inside the request:** a slow or failing SMTP server fails the request, or the email is sent for a change that later rolls back.
- **Send asynchronously after commit (as before):** lost on crash or restart.
- **A message broker:** reliable, but a new service to run for a handful of emails.

## Consequences
- An email is sent if and only if its change was committed, and it survives restarts.
- A crash between sending and marking as sent can send an email twice; the content is safe to repeat.
- One more table and a background worker in the API process.
