# OpenRouter MiniMax cost attribution is not auditable from Aftertalk logs

## Summary

During the Mondopsicologi production investigation on 2026-05-07, the OpenRouter account reported:

- `total_credits`: `115`
- `total_usage`: `115.197586609`

The account is effectively exhausted/overdrawn. The available Aftertalk logs show real MiniMax traffic, but they do not contain per-request token usage, generation IDs, or cost. Because of this, we cannot reconcile OpenRouter spend against Aftertalk sessions from Aftertalk-side evidence alone.

This is an observability and cost-control issue. Aftertalk should persist exact provider billing metadata for every cloud LLM call.

This issue is the billing-specific part of a broader logging/observability gap. The general structured logging SSOT requirement is tracked separately in:

- `issues/2026-05-07-centralized-structured-logging-ssot.md`

## Current Evidence

Runtime model observed in production logs:

- provider adapter: OpenAI-compatible
- OpenRouter model: `minimax/minimax-m2.7`
- current OpenRouter model pricing, fetched from `https://openrouter.ai/api/v1/models` on 2026-05-07:
  - prompt: `0.0000003` credits/token
  - completion: `0.0000012` credits/token
  - context length: `196608`

Journal coverage currently available:

- aftertalk journal starts on `2026-03-17`
- total logged MiniMax `Generate` calls from journal start to `2026-05-07`: `105`
- logged MiniMax `Generate` calls since `2026-05-06 00:00:00`: `57`

Relevant production sessions visible in DB/logs:

### Session `77794a9f-640a-4348-a7c9-04e9b7e30554`

- date: 2026-05-06 / regenerated 2026-05-07
- transcript rows: `240`
- transcript chars: `15753`
- batch plan: `10` incremental batches
- successful 2026-05-07 generation observed with about `15` MiniMax `Generate` calls, including finalization/verification/repair-style extra calls
- later manual retest on 2026-05-07 failed on batch `2/10` after `8` MiniMax `Generate` calls
- during the retest, OpenRouter affordable retries were already down to `max_tokens=510`, `1277`, and `452`, which indicates the credit/key budget was already essentially exhausted before that retest

### Session `94550874-4aeb-4931-a8ff-1328bec55ad8`

- date: 2026-05-07
- transcript rows: `411`
- transcript chars: `14357`
- batch plan: `16` incremental batches
- failed at batch `13/16`
- logs show `16` MiniMax `Generate` calls
- late calls had OpenRouter affordable retries:
  - `max_tokens=7488`
  - `max_tokens=6016`
  - `max_tokens=4817`
  - `max_tokens=3601`
- final error: `context deadline exceeded`

### Earlier failing/short calls

Some previous calls failed with invalid credentials (`sk-test`) or immediate OpenRouter `402` budget errors. These should be considered non-billable or near-zero unless OpenRouter proves otherwise. They are still operationally important because they obscure attribution when no request ID/cost is stored.

## Cost Estimate From Available Segments

This estimate is intentionally conservative. It uses the current MiniMax price and the maximum token budget visible from the service behavior.

Formula:

```text
cost = prompt_tokens * 0.0000003 + completion_tokens * 0.0000012
```

Hard upper bound per observed `Generate` call, assuming the prompt filled the whole MiniMax context and the completion consumed the historical Aftertalk `max_tokens=65536`:

```text
max_input_cost_per_call  = 196608 * 0.0000003 = 0.0589824
max_output_cost_per_call =  65536 * 0.0000012 = 0.0786432
max_total_per_call       = 0.1376256 credits
```

Upper bound for all observed MiniMax calls in retained journals:

```text
105 calls * 0.1376256 = 14.450688 credits
```

Upper bound for the active investigation window since 2026-05-06:

```text
57 calls * 0.1376256 = 7.8446592 credits
```

This is not a realistic estimate; it is a deliberately inflated ceiling. Real prompts for the known sessions are far smaller than the full `196608` token context. The transcript text in the two large sessions is only about `14k-16k` characters each, split across 10-16 batches. Even with templates, accumulated JSON state, finalization, verification, and retries, a practical estimate for the two large sessions is much lower than the hard upper bound.

Approximate practical ranges from the known session sizes:

- `77794a9f-640a-4348-a7c9-04e9b7e30554` successful regeneration: roughly `0.05` to `0.30` credits
- `94550874-4aeb-4931-a8ff-1328bec55ad8` failed generation: roughly `0.05` to `0.35` credits
- `77794a9f-640a-4348-a7c9-04e9b7e30554` failed retest after credit exhaustion: likely below `0.03` credits because affordable output budgets were only a few hundred tokens per call

Even if these practical estimates are too low by several multiples, they do not explain `115.197586609` total usage.

## Conclusion

With the current OpenRouter MiniMax pricing, the observed Aftertalk MiniMax traffic in retained production logs does **not** arithmetically explain the full OpenRouter usage balance.

Possible explanations:

1. The OpenRouter account/key was used by other systems or manually outside this Aftertalk deployment.
2. There are older calls not retained in systemd journals.
3. Historical pricing or provider routing differed materially from current pricing.
4. OpenRouter activity includes other models or endpoints not visible in the Aftertalk journal filter.
5. Aftertalk made calls that are not logged as `OpenAI: Generating response with model ...` due to older code paths.
6. OpenRouter billed hidden reasoning/internal tokens differently than expected.

At the moment, Aftertalk cannot prove or disprove any of these from local logs because it discards the billing metadata returned by OpenRouter.

## Required Fix

Capture and persist cost telemetry for every cloud LLM request.

Minimum fields:

- project/runtime environment
- session ID
- minutes generation phase:
  - batch number
  - finalization
  - verification
  - repair
- provider profile
- adapter/provider name
- requested model
- resolved model/provider when available
- OpenRouter response `id`
- prompt token count
- completion token count
- reasoning token count
- cached token count
- total token count
- cost/usage credits
- request status:
  - success
  - HTTP error
  - timeout
  - parse failure
  - reasoning-only failure
- retry attempt number
- requested `max_tokens`
- affordable retry `max_tokens`, when OpenRouter returns one
- latency
- error class/message, sanitized

OpenRouter documentation says usage metadata is returned in responses and can also be fetched by generation ID:

- `https://openrouter.ai/docs/guides/administration/usage-accounting`
- `https://openrouter.ai/docs/api-reference/generations/get-generation`

Aftertalk should parse/store the response `usage` object immediately. If the provider response only gives an ID, Aftertalk should optionally query `/api/v1/generation?id=...` asynchronously and backfill cost metadata.

## Acceptance Criteria

- Every successful OpenRouter call stores exact usage/cost metadata.
- Failed calls store request ID/cost if OpenRouter returns them.
- Retries are separate cost rows, not collapsed into one opaque generation.
- Session-level minutes records expose total LLM calls, total prompt tokens, total completion tokens, reasoning tokens, and total cost.
- Logs include a sanitized one-line cost summary per LLM call.
- There is a configurable per-session and per-day cloud budget guard.
- When a per-session budget is exceeded, generation fails clearly without fallback to local unless explicitly configured.
- A CLI/admin endpoint can report cloud spend grouped by session, day, model, and profile.

## Operational Recommendation

Until this is implemented:

- rotate the OpenRouter key if external leakage is suspected;
- set an OpenRouter key/account spend limit;
- avoid high-output cloud generation on long sessions without telemetry;
- use the OpenRouter dashboard/activity view as the source of truth for historical attribution.
