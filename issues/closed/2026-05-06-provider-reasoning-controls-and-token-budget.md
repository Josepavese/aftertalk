# Provider reasoning controls and token budgets

Date: 2026-05-06
Priority: High

## Problem

Aftertalk currently assumes that each LLM provider returns the final JSON in the
same response field and that provider defaults are safe for minutes generation.
That is not true for thinking-capable models and OpenAI-compatible routers.

The Mondopsicologi recovery exposed two concrete failures:

1. Ollama with `qwen3.5:2b` returned the useful JSON in the `thinking` field and
   left `response` empty unless the request passed `think: false`.
2. OpenRouter reached the configured cloud model but rejected the request with
   HTTP 402 because the effective output budget allowed up to 65,536 tokens.

## Required Changes

Add provider-level configuration for reasoning/thinking behavior:

- Ollama:
  - support `think: false` in `/api/generate` or `/api/chat` requests;
  - allow this to be configured globally and per profile;
  - default `qwen3.5` local profiles used for structured JSON to no-thinking
    mode unless explicitly overridden.

- OpenAI-compatible / OpenRouter:
  - support configurable `max_tokens` / compatible output-token limit;
  - support provider-specific reasoning controls:
    - `reasoning.effort: "none"` or `reasoning.enabled: false` where supported;
    - `reasoning.exclude: true` / `include_reasoning: false` only as an output
      hiding option, not as the only cost-control mechanism;
  - allow these options per LLM profile, not only globally.

## Acceptance Criteria

- A local `qwen3.5:2b` smoke generation returns parseable JSON in the field
  consumed by Aftertalk.
- An OpenRouter profile can be configured with a safe output token cap.
- An OpenRouter profile can explicitly disable or minimize reasoning for models
  that support it.
- If a provider/model ignores the requested reasoning control, the logs must
  make that visible enough for operators to diagnose cost/latency issues.
- Tests cover at least:
  - Ollama request body includes `think:false` when configured;
  - OpenAI-compatible request body includes configured reasoning and token caps;
  - empty final content with non-empty reasoning/thinking is treated as a
    provider incompatibility, not silently retried as an empty answer.

## Notes

OpenRouter's reasoning controls differ from Ollama's `think` field. The
framework should expose a normalized Aftertalk config, but each provider adapter
must translate it into the provider-specific request body.
