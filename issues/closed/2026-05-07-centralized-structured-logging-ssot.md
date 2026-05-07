# Centralized structured logging SSOT with retention, rotation, and debug inspection

## Summary

Aftertalk needs a real logging subsystem, not scattered formatted strings and process stdout prints.

The current code already has `internal/logging` backed by `zap`, but most runtime logs are still emitted through `Infof/Warnf/Errorf` string interpolation. That makes logs hard to query, correlate, retain, rotate, redact, and use for incident reconstruction.

This issue is broader than the OpenRouter billing investigation, but that incident exposed the problem clearly: Aftertalk made real cloud LLM calls, yet we cannot reconstruct exact per-request cost, generation IDs, tokens, retries, or session linkage from local logs.

Related issue:

- `issues/2026-05-07-openrouter-minimax-cost-attribution.md`

## Current State

Observed in the repository:

- `internal/logging/logger.go` initializes a global `zap.SugaredLogger`.
- Runtime logging is mostly `logging.Infof("... %s ...", value)` rather than structured fields.
- Output paths are hardcoded to:
  - `stdout`
  - `stderr`
- No file sink is configured by Aftertalk.
- No application-level rotation/retention/cutoff policy exists.
- No central log schema exists.
- No mandatory correlation fields exist.
- No request/session/generation context propagation exists.
- No sanitizer/redaction layer is enforced.
- Some non-runtime/installer/scripts still use direct print calls:
  - Go `fmt.Println`, `fmt.Printf`, `fmt.Fprintln`
  - Python `print`
  - documentation examples with `console.log`

Some CLI stdout output is legitimate user-facing output, for example `--version`, `--dump-defaults`, or interactive installer prompts. The problem is diagnostic/runtime output and operational events bypassing the centralized logger.

## Problem

Aftertalk is a production service that handles:

- WebRTC session lifecycle;
- STT calls;
- LLM calls;
- cloud provider failures;
- local model fallback/readiness;
- webhook delivery/retry;
- minutes generation and regeneration;
- billing-sensitive cloud usage.

For these flows, plain interpolated log messages are insufficient.

During incidents we need to answer questions like:

- Which session triggered this provider call?
- Which profile was used?
- Which model was requested and which upstream model/provider handled it?
- Which retry attempt failed?
- How long did each call take?
- How many tokens were consumed?
- What did OpenRouter charge?
- Was reasoning enabled, disabled, or returned unexpectedly?
- Which webhook was delivered, retried, or failed?
- Which request/user/IP triggered a regenerate?
- Did the event happen before or after a deploy/restart?

Today these answers require manual `grep` and partial inference. Some answers are impossible because the data is never logged or persisted.

## Required Architecture

Create a centralized structured logging subsystem treated as SSOT for operational diagnostics.

### 1. Central logging configuration

Add SSOT configuration fields for logging, for example:

```yaml
logging:
  level: info
  format: json
  output:
    stdout: true
    file:
      enabled: true
      path: /var/log/aftertalk/aftertalk.jsonl
  rotation:
    max_size_mb: 100
    max_age_days: 30
    max_backups: 20
    compress: true
  retention:
    delete_after_days: 90
    emergency_cutoff_size_mb: 2048
  redaction:
    enabled: true
    fields:
      - api_key
      - token
      - authorization
      - secret
      - password
      - webhook_payload
      - transcript_text
```

The installer/deployer must materialize these fields into runtime config without project-specific patches.

### 2. Structured event schema

Introduce stable event names and structured fields.

Every runtime log should have at least:

- `ts`
- `level`
- `event`
- `component`
- `service`
- `env`
- `version`
- `release`
- `request_id`, when available
- `session_id`, when available
- `participant_id`, when available
- `minutes_id`, when available
- `provider_profile`, when available
- `provider`, when available
- `model`, when available
- `attempt`, when available
- `duration_ms`, when available
- `error_code`, when available
- `error`, sanitized

Event examples:

- `api.request.started`
- `api.request.completed`
- `session.created`
- `session.ended`
- `session.recovery.started`
- `audio.chunk.received`
- `stt.request.started`
- `stt.request.completed`
- `stt.request.failed`
- `llm.request.started`
- `llm.request.completed`
- `llm.request.failed`
- `llm.request.retry_scheduled`
- `llm.provider_budget_rejected`
- `minutes.batch.started`
- `minutes.batch.completed`
- `minutes.finalize.started`
- `minutes.verify.started`
- `webhook.delivery.started`
- `webhook.delivery.completed`
- `webhook.delivery.failed`
- `config.loaded`
- `deploy.version.loaded`

### 3. Replace formatted runtime logs

Replace runtime diagnostic logging such as:

```go
logging.Infof("Generating minutes for session %s (template=%s)", sessionID, tmpl.ID)
```

with structured logging:

```go
logging.Info("minutes.generation.started",
    "session_id", sessionID,
    "template_id", tmpl.ID,
)
```

or an equivalent typed helper API:

```go
logging.Event(ctx, "minutes.generation.started").
    Session(sessionID).
    String("template_id", tmpl.ID).
    Info()
```

The exact API is a team decision, but the result must be structured JSON logs with stable keys.

### 4. Replace direct print/log usage

Runtime/service diagnostics must not use:

- `fmt.Print*`
- `log.Print*`
- raw `println`
- Python `print`
- JavaScript `console.log` in production SDK/runtime code

Exceptions are allowed only for intentional user-facing CLI output, such as:

- `aftertalk --version`
- `aftertalk --dump-defaults`
- interactive installer prompts
- test scripts explicitly designed for human console output
- documentation snippets

Even when CLI output remains user-facing, diagnostic events from the same command should go through the central logger.

### 5. File rotation, retention, and cutoff

Logging must not rely only on systemd/journald retention.

Add an application-level log file sink with:

- size-based rotation;
- age-based retention;
- max backup count;
- compression of old logs;
- hard emergency cutoff to prevent filling the disk;
- documented defaults suitable for small VPS deployments.

The system should degrade safely if the file sink cannot be opened:

- continue logging to stdout/stderr;
- emit one structured warning;
- do not crash unless config explicitly says logging file sink is mandatory.

### 6. Inspection and debug tooling

Add a supported way to inspect logs without fragile `grep` pipelines.

Possible options:

- CLI:
  - `aftertalk logs tail`
  - `aftertalk logs query --session <id>`
  - `aftertalk logs query --event llm.request.completed`
  - `aftertalk logs usage --from ... --to ...`
- admin/internal endpoint protected by API key;
- JSONL files with documented `jq` recipes.

Minimum useful queries:

- all events for a session;
- all failed LLM calls;
- all cloud provider calls grouped by session/model;
- all webhook failures;
- all stuck/processing session recoveries;
- per-day LLM cost summary once provider usage is implemented.

### 7. Request and workflow correlation

Introduce request/workflow context propagation.

Required correlation IDs:

- inbound API `request_id`;
- generated `workflow_id` for long async flows;
- `session_id`;
- `minutes_id`;
- LLM `generation_id`, when the provider returns one;
- webhook event ID;
- deploy/release version.

Async work must keep correlation fields when leaving the original HTTP request context.

### 8. Redaction and data safety

The logger must sanitize sensitive fields before output.

Never log raw values for:

- API keys;
- bearer tokens;
- webhook secrets;
- authorization headers;
- passwords;
- full transcript text;
- full generated minutes;
- raw provider payloads unless explicitly enabled in a local debug profile.

For clinical/product debugging, log metadata and bounded previews only:

- character counts;
- segment counts;
- batch counts;
- hashes;
- short sanitized snippets only when enabled.

### 9. Provider usage integration

For OpenRouter/OpenAI-compatible LLM calls, logging must include billing metadata when available:

- response `id`;
- prompt tokens;
- completion tokens;
- reasoning tokens;
- cached tokens;
- total tokens;
- cost;
- upstream provider/model if returned;
- retry attempt;
- requested `max_tokens`;
- adjusted affordable `max_tokens`;
- latency;
- failure class.

This can initially be logged and later persisted in DB, but the fields and event names should be stable from the beginning.

### 10. Build/test enforcement

Add lint/test checks to prevent regression:

- forbid `fmt.Print*` and `log.Print*` in runtime packages;
- allow explicit exceptions for CLI/user-facing output with comments;
- forbid direct `zap` usage outside `internal/logging`;
- require structured event names for service logs;
- add tests for redaction;
- add tests for rotation config parsing/materialization.

## Acceptance Criteria

- Logging config is fully driven by SSOT runtime configuration.
- Runtime logs are JSON structured by default in production.
- File output, rotation, retention, compression, and emergency cutoff are configurable.
- Existing runtime formatted logs are migrated to structured fields.
- Direct print/log usage is removed from runtime paths or explicitly justified as user-facing CLI output.
- Logs include stable event names.
- API requests include request IDs.
- Session, minutes, webhook, STT, and LLM flows carry correlation IDs.
- Sensitive values are redacted by default.
- LLM provider calls log usage/cost metadata where available.
- A supported CLI or documented `jq` workflow can inspect logs by session, event, provider, and date.
- Tests/lints prevent new unstructured runtime logging from being introduced.

## Operational Value

This would make incidents diagnosable without guessing.

For example, the OpenRouter credit issue would be answerable directly:

- list all LLM calls in the period;
- group by session/model/provider profile;
- sum exact provider cost;
- identify retries and failed calls;
- distinguish Aftertalk production traffic from external/manual key usage.

Without this, production support remains dependent on incomplete grep-based reconstruction.
