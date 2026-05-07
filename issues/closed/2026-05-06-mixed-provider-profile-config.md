# Mixed provider profile configuration must materialize credentials

Date: 2026-05-06
Priority: High

## Problem

Aftertalk supports per-session LLM profiles, but the installer/config writer can
produce incomplete YAML when the default provider and a named profile use
different providers.

Mondopsicologi uses:

- default/local LLM profile: `ollama`
- Premium/cloud LLM profile: `openai` through OpenRouter

The generated runtime YAML included the cloud profile name and model, but did
not materialize the shared OpenAI-compatible credentials/base URL because the
top-level `llm_provider` was `ollama`.

Result: the application could select the `cloud` profile correctly, while the
underlying provider still lacked the real OpenAI-compatible runtime settings.

## Required Changes

- The installer/config writer must materialize shared provider blocks for every
  provider referenced by any profile, not only for the top-level provider.
- The config schema should support provider settings per profile, including:
  - API key;
  - base URL;
  - model;
  - request timeout;
  - max output tokens;
  - reasoning/thinking options.
- Add validation that fails fast when a profile references a provider without
  the required provider credentials.
- `aftertalk --version` or `/v1/ready` should expose enough profile readiness
  detail for deploy verification without printing secrets.

## Acceptance Criteria

- A config with default `ollama` plus cloud `openai` profile generates YAML that
  contains both `llm.ollama` and `llm.openai` runtime blocks.
- A config with missing cloud credentials fails installer verification or marks
  the cloud profile unhealthy.
- Secrets are never printed in logs, health output, or generated diagnostics.
- Tests cover mixed-provider profile rendering.

## Production Context

Mondopsicologi has a local deploy-side mitigation that patches the generated
YAML after installer execution. This must become native Aftertalk behavior so
tenant deploy scripts do not need to repair framework output.
