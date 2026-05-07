# notify_pull purge logs context canceled after successful webhook delivery

## Context

During production regeneration on 2026-05-07, minutes were generated successfully and notify-pull webhook events were delivered.

Immediately around successful delivery, the service logged errors like:

```text
PurgeMinutes: get minutes <minutes_id>: failed to get minutes: context canceled
```

Observed sessions:

- `b1837e16-1ac0-46a1-b68f-bab1e3a0f55c`
- `4e351ae5-03ba-417a-a609-3b8a13110265`
- `77794a9f-640a-4348-a7c9-04e9b7e30554`

All three ended as:

- session status: `completed`
- minutes status: `ready`
- provider: `openai`
- webhook event status: `delivered`

## Problem

The error appears after a successful webhook delivery and did not block the user-facing result, but it pollutes production logs and makes incident triage harder.

The likely cause is that purge work is using a request-scoped context that can already be canceled after the pull/webhook flow completes.

## Proposal

Review the notify-pull purge path and ensure cleanup uses an appropriate bounded background context instead of a canceled request context.

The code should also distinguish between:

- cleanup skipped because context is canceled;
- cleanup failed because data is missing or DB operation failed;
- cleanup intentionally not executed because `delete_on_pull` is disabled.

## Acceptance Criteria

- Successful webhook/pull delivery does not emit `PurgeMinutes: ... context canceled`.
- Cleanup still has a bounded timeout.
- Logs make cleanup failures actionable and do not look like minutes generation failures.
- Regression test covers notify-pull delivery followed by purge after the caller context is canceled.
