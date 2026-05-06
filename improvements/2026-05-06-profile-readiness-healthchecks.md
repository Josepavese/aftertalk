# Profile readiness and provider health checks

Date: 2026-05-06
Priority: Medium-High

## Problem

`aftertalk.service` can be active and `/v1/ready` can pass while one or more
configured provider profiles are unusable.

For Premium/cloud flows this is dangerous: the base application can select the
right profile, but generation still fails later because credentials, base URL,
token budget, or provider options are invalid.

## Required Changes

- Add profile-level readiness checks for STT and LLM profiles.
- Checks must validate configuration shape without exposing secrets.
- Optional deep checks should be available during deploy:
  - provider auth check;
  - minimal non-sensitive generation check;
  - model availability check for Ollama.
- `/v1/ready` should support a detailed mode for operators/deploy scripts, for
  example `?details=1`, returning profile names and health states.
- The installer/deploy verifier should fail when required profiles are unhealthy.

## Acceptance Criteria

- If a configured cloud profile has missing credentials, readiness details show
  that profile as unhealthy.
- If a local Ollama model is missing, readiness details show that profile as
  unhealthy.
- Deploy verification can require specific profiles, for example
  `local,cloud`, and fail if either is unusable.
- Secret values are redacted in every output.

## Production Context

Mondopsicologi needed Premium/cloud to work, but the runtime problem was only
visible after a real patient session reached minutes generation.
