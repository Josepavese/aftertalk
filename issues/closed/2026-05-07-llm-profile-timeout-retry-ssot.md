# LLM profile timeout and retry budget should be profile-scoped SSOT

## Context

During the Mondopsicologi production recovery on 2026-05-07, failed cloud minutes were regenerated through the Aftertalk API with `llm_profile=cloud`.

The cloud profile used the OpenAI-compatible provider through OpenRouter with model `minimax/minimax-m2.7`.

Short sessions completed correctly. The long session `77794a9f-640a-4348-a7c9-04e9b7e30554` also completed, but late batches hit repeated read timeouts at the current 120s request timeout:

- batch 8: first attempt timed out, retry succeeded;
- batch 10: multiple attempts timed out, later retry succeeded;
- finalization completed;
- webhook notification was delivered;
- no fallback to Ollama occurred, which is correct for this flow.

## Problem

The current request timeout can be raised from deployment configuration, but that is not the cleanest ownership boundary.

Timeout and retry behavior are provider/model characteristics. Local Ollama and high-latency cloud profiles have different operational needs, so this should live in the same SSOT area as the provider profile configuration.

## Proposal

Make timeout/retry budgets explicitly profile-scoped in the provider SSOT, for example:

- `llm.profiles.cloud.request_timeout`
- `llm.profiles.cloud.retry.max_attempts`
- `llm.profiles.cloud.retry.initial_backoff`
- `llm.profiles.cloud.retry.max_backoff`
- optional end-to-end `llm.profiles.cloud.generation_timeout`

The installer/env bridge should materialize these values directly into runtime config without project-specific post-processing.

## Acceptance Criteria

- Cloud and local LLM profiles can have different timeout values.
- Retry policy is either profile-scoped or intentionally global and documented.
- Installer/config writer preserves the profile timeout/retry fields.
- Tests cover a cloud profile with a higher timeout than the local profile.
- Documentation explains recommended values for high-latency cloud models.
