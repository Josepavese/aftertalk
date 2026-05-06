# Session error handling, recovery, and regeneration

Date: 2026-05-06
Priority: High

## Problem

If minutes generation times out or a provider call fails late, Aftertalk can
leave the session in an ambiguous state unless every error path updates the DB
successfully.

During the Mondopsicologi incident, a local generation timeout attempted to mark
the session as failed using the already-expired generation context. That DB
update failed, leaving:

- `sessions.status = processing`
- `minutes.status = pending`
- placeholder minutes content

The integrating application kept showing "generation in progress" because there
was no terminal state to consume.

## Required Changes

- Failure-state writes must use a fresh bounded context, not the canceled
  generation context.
- `minutes.status` and `sessions.status` must move to terminal `error` on every
  unrecoverable generation failure.
- Add a first-class regenerate/retry operation for sessions with existing
  transcriptions:
  - API endpoint;
  - CLI/admin command;
  - safe idempotency semantics.
- Add stuck-processing detection:
  - detect sessions/minutes older than the configured generation timeout;
  - mark them as error or enqueue a bounded retry;
  - log an actionable diagnostic with session ID, profile, provider, and age.

## Acceptance Criteria

- A forced LLM timeout leaves the session in `error`, never indefinitely in
  `processing`.
- A forced provider 4xx/5xx leaves minutes in `error` with enough diagnostic
  metadata for operators.
- An operator can retry/regenerate a failed session without manual SQLite edits.
- Recovery does not duplicate minutes rows and does not emit duplicate webhooks
  unless explicitly requested.
- Tests cover timeout, provider failure, retry, and concurrent retry races.

## Production Context

Manual recovery on Mondopsicologi required direct SQLite updates plus another
`POST /v1/sessions/{id}/end`. That should not be an operational requirement.
