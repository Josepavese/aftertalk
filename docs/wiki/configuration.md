# Configuration

Configuration is loaded in order (each subsequent source overrides the previous):
1. Default values (`config.Default()`)
2. YAML file (if specified with `--config config.yaml`)
3. Environment variables with prefix `AFTERTALK_`

Env key convention: `AFTERTALK_` + koanf path with `_` instead of `.`
Example: `webhook.pull_base_url` → `AFTERTALK_WEBHOOK_PULL_BASE_URL`

> **Validation**: the server will not start if `JWT_SECRET` or `API_KEY` are at their default values.

---

## Database

| Env key | YAML | Default | Notes |
|---|---|---|---|
| `AFTERTALK_DATABASE_PATH` | `database.path` | `./aftertalk.db` | SQLite file path |

---

## HTTP

| Env key | YAML | Default |
|---|---|---|
| `AFTERTALK_HTTP_PORT` | `http.port` | `8080` |
| `AFTERTALK_HTTP_HOST` | `http.host` | `0.0.0.0` |

---

## Security

### JWT
| Env key | YAML | Default | Notes |
|---|---|---|---|
| `AFTERTALK_JWT_SECRET` | `jwt.secret` | *(none)* | **Required**, min 32 chars |
| `AFTERTALK_JWT_ISSUER` | `jwt.issuer` | `aftertalk` | |
| `AFTERTALK_JWT_EXPIRATION` | `jwt.expiration` | `2h` | Go duration: `2h`, `30m` |

### API Key
| Env key | YAML | Default | Notes |
|---|---|---|---|
| `AFTERTALK_API_KEY` | `api.key` | *(none)* | **Required** in production |

### CORS
```yaml
api:
  cors:
    allowed_origins: ["https://app.example.com"]  # default: ["*"]
    allowed_methods: ["GET","POST","PUT","DELETE","OPTIONS"]
    allowed_headers: ["Authorization","Content-Type","X-API-Key","X-Request-ID","X-User-ID"]
    allow_credentials: false
```

### Rate Limiting
```yaml
api:
  rate_limit:
    enabled: true
    requests_per_minute: 100  # per IP
```

---

## STT (Speech-to-Text)

| Env key | YAML | Default | Values |
|---|---|---|---|
| `AFTERTALK_STT_PROVIDER` | `stt.provider` | `google` | `google`, `aws`, `azure`, `whisper-local`, `stub` |

### Google Cloud STT
```yaml
stt:
  provider: google
  google:
    credentials_path: /path/to/service-account.json
```

### AWS Transcribe
```yaml
stt:
  provider: aws
  aws:
    access_key_id: AKIA...
    secret_access_key: secret
    region: us-east-1
```

### Azure Speech
```yaml
stt:
  provider: azure
  azure:
    key: your-key
    region: eastus
```

### Local Whisper
```yaml
stt:
  provider: whisper-local
  whisper_local:
    url: http://localhost:9000   # required
    model: base
    language: en
    response_format: verbose_json
```

### Stub (no real transcription)
```yaml
stt:
  provider: stub
```
Returns a placeholder segment `[stub: Xms of audio from role]`. Useful for development.

---

## LLM (Minutes Generation)

| Env key | YAML | Default | Values |
|---|---|---|---|
| `AFTERTALK_LLM_PROVIDER` | `llm.provider` | `openai` | `openai`, `anthropic`, `azure`, `ollama`, `stub` |

### OpenAI
```yaml
llm:
  provider: openai
  openai:
    api_key: sk-...
    model: gpt-4o          # recommended; gpt-4 works too
    base_url: ""           # optional: override API base (e.g. https://openrouter.ai/api)
```

#### OpenRouter (OpenAI-compatible)
Any OpenAI-compatible API can be used via `base_url`:
```yaml
llm:
  provider: openai
  openai:
    api_key: sk-or-v1-...
    model: openai/gpt-4o-mini
    base_url: https://openrouter.ai/api
```

### Anthropic
```yaml
llm:
  provider: anthropic
  anthropic:
    api_key: sk-ant-...
    model: claude-sonnet-4-6
```
> Note: the Anthropic provider uses `anthropic-version: 2023-06-01` hardcoded.

### Azure OpenAI
```yaml
llm:
  provider: azure
  azure:
    api_key: your-key
    endpoint: https://your-resource.openai.azure.com
    deployment: gpt-4
```

### Ollama (local)
```yaml
llm:
  provider: ollama
  ollama:
    base_url: http://localhost:11434
    model: llama3
```

### Stub
```yaml
llm:
  provider: stub
```
Generates a synthetic summary from the transcription text without API calls.
> Note: the Stub is optimized for the `therapy` template. For other templates, sections may not match.

### Request timeouts
The HTTP timeout for cloud LLM providers can be overridden per provider:

```yaml
llm:
  openai:
    request_timeout: 120s
  anthropic:
    request_timeout: 120s
  azure:
    request_timeout: 120s
```

If omitted, the built-in default is `120s`.

---

## Provider Profiles (per-session routing)

Named profiles allow different sessions to use different STT or LLM providers without restarting the server. Profiles are built once at startup; `Get(profileName)` is O(1).

### STT Profiles
```yaml
stt:
  default_profile: local        # used when session omits stt_profile
  whisper_local:
    url: http://localhost:9001
  google:
    credentials_path: /path/to/key.json
  profiles:
    local:
      provider: whisper-local
    cloud:
      provider: google
```

### LLM Profiles
```yaml
llm:
  default_profile: local        # used when session omits llm_profile
  ollama:
    base_url: http://localhost:11434
    model: qwen2.5:3b
  openai:
    api_key: sk-...
    model: gpt-4o-mini
  profiles:
    local:
      provider: ollama
    cloud:
      provider: openai
      model: gpt-4o             # optional model override
```

### Session creation with profile selection
```json
POST /v1/sessions
{
  "participant_count": 2,
  "participants": [...],
  "stt_profile": "cloud",
  "llm_profile": "cloud"
}
```
If `stt_profile` / `llm_profile` are omitted the configured `default_profile` is used.
If no profiles are defined, the legacy single-provider mode is used (profile name `"default"`).

---

## Webhook

See [webhook.md](webhook.md) for full documentation.

```yaml
webhook:
  url: https://your-app.example.com/webhook
  timeout: 30s
  mode: push           # "push" (default) or "notify_pull"
  secret: ""           # HMAC secret for notify_pull
  token_ttl: 1h        # pull token TTL
  pull_base_url: ""    # public Aftertalk URL for notify_pull
  delete_on_pull: null # default true for notify_pull
```

---

## Processing

```yaml
processing:
  max_concurrent_transcriptions: 10
  max_concurrent_minutes_generations: 5
  transcription_timeout: 10m
  minutes_generation_timeout: 5m
  llm_max_retries: 3
  llm_initial_backoff: 1s
  llm_max_backoff: 10s
  transcription_queue_size: 100
  chunk_size_ms: 15000      # audio chunk size for transcription (ms)
  minutes_incremental: true
  minutes_batch_max_segments: 24
  minutes_batch_max_chars: 6000
  minutes_max_summary_phases: 8
  minutes_max_citations: 12
```

`chunk_size_ms` controls how many ms of accumulated audio triggers a transcription. VAD (Voice Activity Detection) may trigger earlier on extended silence.

### Incremental minutes generation

Minutes generation is incremental by default. Aftertalk keeps a compact structured
state and updates it batch by batch instead of materializing the full session in
one LLM request.

| Key | Default | Meaning |
|---|---|---|
| `processing.minutes_incremental` | `true` | Enables iterative minutes reduction |
| `processing.minutes_batch_max_segments` | `24` | Max transcript lines per LLM batch |
| `processing.minutes_batch_max_chars` | `6000` | Max transcript characters per LLM batch |
| `processing.minutes_max_summary_phases` | `8` | Max entries in `summary.phases` after normalization |
| `processing.minutes_max_citations` | `12` | Max citations kept in the final state |

The generated minutes include a top-level `summary` object:

```json
{
  "summary": {
    "overview": "Concise summary of the conversation",
    "phases": [
      {
        "title": "Opening",
        "summary": "Greeting and initial alignment",
        "start_ms": 0,
        "end_ms": 60000
      }
    ]
  }
}
```

---

## WebRTC / ICE

```yaml
webrtc:
  ice_provider: static    # static, embedded, twilio, xirsys, metered
  ice_servers:
    - urls: ["stun:stun.l.google.com:19302"]
```

### ICE Provider: embedded TURN
```yaml
webrtc:
  ice_provider: embedded
  turn:
    enabled: true
    listen_addr: "0.0.0.0:3478"
    public_ip: ""        # auto-detect if empty
    realm: aftertalk
    auth_ttl: 86400
    enable_udp: true
    enable_tcp: true
```

### ICE Provider: Twilio
```yaml
webrtc:
  ice_provider: twilio
  twilio:
    account_sid: ACxxx
    auth_token: xxx
```

### ICE Provider: Xirsys
```yaml
webrtc:
  ice_provider: xirsys
  xirsys:
    ident: your-ident
    secret: your-secret
    channel: your-channel
```

### ICE Provider: Metered.ca
```yaml
webrtc:
  ice_provider: metered
  metered:
    app_name: your-app
    api_key: your-key
```

---

## Session & Retention

```yaml
session:
  max_duration: 2h    # 0 = disabled (no auto-timeout)
  max_participants_per_session: 10

retention:
  transcription_days: 90
  minutes_days: 90
  webhook_events_days: 30
```

### Session inactivity auto-end (`inactivity_timeout`)

When `inactivity_timeout` is set, a session is automatically ended after the configured period
of silence (no audio received from any participant). The timer resets on every audio chunk.

**Restart safety**: on process startup, inactivity timers are restored for all active sessions
using the last transcription timestamp from the DB. Sessions already overdue are ended
immediately. This means the inactivity trigger is reliable across restarts, not just in-memory.

| Env key | YAML | Default |
|---|---|---|
| `AFTERTALK_SESSION_INACTIVITY_TIMEOUT` | `session.inactivity_timeout` | `10m` |

**Example:**
```yaml
session:
  inactivity_timeout: 20m   # end session after 20 min of silence
```

---

### Session auto-timeout (`max_duration`)

When `max_duration` is set to a non-zero value, a background **session reaper** runs every 5 minutes and automatically ends any `active` session whose age exceeds the configured duration.

- The check is DB-based (`created_at` column), so it survives process restarts — sessions that were active before a restart are still reaped on the next sweep.
- Closing a session triggers the normal `EndSession` flow: remaining audio is transcribed, minutes are generated, and the webhook is delivered exactly as if the client had called `POST /v1/sessions/{id}/end`.
- Set `max_duration: 0` to disable the reaper entirely (previous behaviour — sessions only end when explicitly closed by the client).

**Example for MondoPsicologi (70-minute therapy sessions):**
```yaml
session:
  max_duration: 1h10m
```

| Env key | YAML | Default | Notes |
|---|---|---|---|
| `AFTERTALK_SESSION_MAX_DURATION` | `session.max_duration` | `2h` | Go duration string; `0` disables reaper |
| `AFTERTALK_SESSION_MAX_PARTICIPANTS_PER_SESSION` | `session.max_participants_per_session` | `10` | |

## Get the default YAML config

```bash
./bin/aftertalk --dump-defaults > config.yaml
```
