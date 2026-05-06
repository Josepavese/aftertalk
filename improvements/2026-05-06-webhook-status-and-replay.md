# Minutes webhook status, error notification, and replay

Date: 2026-05-06
Priority: Medium-High

## Problem

Integrating applications need a terminal signal when minutes generation fails,
not only when minutes are ready.

In the Mondopsicologi incident, the application DB had no ready minutes payload
and the UI kept showing an in-progress state. Even after Aftertalk correctly
marked the session as `error`, the integration still needs a clean way to
receive, store, inspect, and potentially replay that terminal state.

## Required Changes

- Define webhook notifications for terminal failure states, not only ready
  minutes.
- Include a safe error code/category in the notification:
  - provider_auth;
  - provider_quota_or_budget;
  - provider_timeout;
  - parse_error;
  - internal_timeout;
  - unknown.
- Add an operator-facing webhook event list/replay mechanism:
  - list events by session/minutes ID;
  - replay a ready or error notification;
  - show attempts, last status, and next retry time.
- Ensure notify-pull mode has a clear behavior for error states where no minutes
  payload exists.

## Acceptance Criteria

- A generation failure emits a signed webhook notification with a terminal error
  status.
- The integrating application can stop polling "in progress" and show/retry an
  error state.
- Operators can replay a terminal notification without manual DB edits.
- Webhook retry history is inspectable without exposing PHI or secrets.

## Production Context

Mondopsicologi currently needs manual inspection to distinguish "still
generating" from "failed in Aftertalk/provider". This should be part of the
framework contract.
