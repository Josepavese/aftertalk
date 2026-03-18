# Improvement 21 — Per-Session Provider Profiles (STT & LLM)

**Status:** In progress
**Priority:** High
**Area:** Config, Core, API, UI

---

## Problem

STT and LLM providers are selected globally at startup via a single `STT_PROVIDER` /
`LLM_PROVIDER` env var. Every session uses the same provider regardless of quality
requirements, cost budget, or user tier. There is no way to route specific sessions
to a faster (cloud) or cheaper (local) backend at creation time.

---

## Solution

Introduce **named provider profiles** for STT and LLM. A profile maps a human-readable
name (e.g. `local`, `cloud`) to a provider selection. At session creation the caller
can optionally specify which profile to use; otherwise the configured default applies.

### Key design decisions

1. **SSOT preserved** — credentials and URLs remain in their existing config sections
   (`stt.whisper_local.url`, `llm.ollama.base_url`, etc.). Profiles only declare
   *which* provider to select, optionally overriding the model.
2. **No per-session provider construction** — a `ProviderRegistry` is built once at
   startup with one provider instance per profile. `Get(profileName)` is O(1).
3. **Backward compatible** — if no profiles are defined, behaviour is unchanged.
   The legacy `stt.provider` / `llm.provider` fields continue to work as the default.
4. **Caller chooses profile by name** — the API consumer (e.g. MondoPsicologi) passes
   `stt_profile: "cloud"` to give premium users higher-quality transcription without
   knowing any credentials.

---

## Config schema (YAML)

```yaml
stt:
  provider: whisper-local          # legacy default (used when no profiles defined)
  default_profile: local           # used when session omits stt_profile
  profiles:
    local:
      provider: whisper-local
    cloud:
      provider: google             # credentials from stt.google.*

llm:
  provider: ollama                 # legacy default
  default_profile: local
  profiles:
    local:
      provider: ollama
      model: qwen2.5:3b            # overrides llm.ollama.model
    cloud:
      provider: openai
      model: gpt-4o                # overrides llm.openai.model
```

---

## API changes

### POST /v1/sessions
```json
{
  "participant_count": 2,
  "template_id": "therapy",
  "stt_profile": "cloud",     // optional, defaults to stt.default_profile
  "llm_profile": "cloud"      // optional, defaults to llm.default_profile
}
```

### GET /v1/sessions/{id}
Response now includes:
```json
{
  "stt_profile": "cloud",
  "llm_profile": "local"
}
```

### GET /demo/config
Returns available profiles so the UI can show a selector:
```json
{
  "stt_profiles": ["local", "cloud"],
  "llm_profiles": ["local", "cloud"],
  "default_stt_profile": "local",
  "default_llm_profile": "local"
}
```

---

## Implementation plan

1. **`internal/config/config.go`** — add `STTProfileConfig`, `LLMProfileConfig`,
   `DefaultProfile string`, `Profiles map[string]...` to `STTConfig`/`LLMConfig`
2. **`internal/ai/stt/registry.go`** — `STTRegistry` struct + `NewSTTRegistry(cfg)` +
   `Get(profile) STTProvider`
3. **`internal/ai/llm/registry.go`** — same for LLM
4. **`internal/core/session/entity.go`** — add `STTProfile`, `LLMProfile` fields
5. **`internal/core/session/service.go`** — `CreateSession` accepts profiles; processing
   calls `registry.Get(session.STTProfile)` / `registry.Get(session.LLMProfile)`
6. **`internal/storage/sqlite/db.go`** / **`cmd/aftertalk/main.go`** — add migration:
   `ALTER TABLE sessions ADD COLUMN stt_profile TEXT DEFAULT ''`
   `ALTER TABLE sessions ADD COLUMN llm_profile TEXT DEFAULT ''`
7. **`internal/core/session/repository.go`** — persist/load new columns
8. **`internal/api/handler/session.go`** — accept `stt_profile`/`llm_profile` in
   `CreateSessionRequest`; expose in `GET /demo/config` response
9. **`cmd/test-ui/index.html`** — profile selector dropdowns at session creation
10. **Docs** — update `MEMORY.md`, wiki, README

---

## Use cases enabled

| Scenario | stt_profile | llm_profile |
|---|---|---|
| Free user | `local` | `local` |
| Premium user | `cloud` | `cloud` |
| High-accuracy transcription, fast summary | `cloud` | `local` |
| Debug / dev session | `local` | `local` |
| Production default (unspecified) | *(default_profile)* | *(default_profile)* |
