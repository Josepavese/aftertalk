# Configuration

La configurazione viene caricata in ordine (il successivo sovrascrive):
1. Valori di default (`config.Default()`)
2. File YAML (se specificato con `--config config.yaml`)
3. Variabili d'ambiente con prefisso `AFTERTALK_`

Env key convention: `AFTERTALK_` + percorso koanf con `_` al posto di `.`
Es: `webhook.pull_base_url` → `AFTERTALK_WEBHOOK_PULL_BASE_URL`

> **Validazione**: il server non parte se `JWT_SECRET` o `API_KEY` sono ai valori di default.

---

## Database

| Chiave env | YAML | Default | Note |
|---|---|---|---|
| `AFTERTALK_DATABASE_PATH` | `database.path` | `./aftertalk.db` | Path del file SQLite |

---

## HTTP

| Chiave env | YAML | Default |
|---|---|---|
| `AFTERTALK_HTTP_PORT` | `http.port` | `8080` |
| `AFTERTALK_HTTP_HOST` | `http.host` | `0.0.0.0` |

---

## Sicurezza

### JWT
| Chiave env | YAML | Default | Note |
|---|---|---|---|
| `AFTERTALK_JWT_SECRET` | `jwt.secret` | *(nessuno)* | **Obbligatorio**, min 32 char |
| `AFTERTALK_JWT_ISSUER` | `jwt.issuer` | `aftertalk` | |
| `AFTERTALK_JWT_EXPIRATION` | `jwt.expiration` | `2h` | Formato Go: `2h`, `30m` |

### API Key
| Chiave env | YAML | Default | Note |
|---|---|---|---|
| `AFTERTALK_API_KEY` | `api.key` | *(nessuno)* | **Obbligatorio** in produzione |

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

| Chiave env | YAML | Default | Valori |
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

### Whisper locale
```yaml
stt:
  provider: whisper-local
  whisper_local:
    url: http://localhost:9000   # obbligatorio
    model: base
    language: it
    response_format: verbose_json
```

### Stub (nessuna trascrizione reale)
```yaml
stt:
  provider: stub
```
Restituisce un segmento placeholder `[stub: Xms di audio da role]`. Utile per sviluppo.

---

## LLM (Minutes Generation)

| Chiave env | YAML | Default | Valori |
|---|---|---|---|
| `AFTERTALK_LLM_PROVIDER` | `llm.provider` | `openai` | `openai`, `anthropic`, `azure`, `ollama`, `stub` |

### OpenAI
```yaml
llm:
  provider: openai
  openai:
    api_key: sk-...
    model: gpt-4o          # raccomandato; gpt-4 funziona
```

### Anthropic
```yaml
llm:
  provider: anthropic
  anthropic:
    api_key: sk-ant-...
    model: claude-sonnet-4-6
```
> Nota: il provider Anthropic usa `anthropic-version: 2023-06-01` hardcoded.

### Azure OpenAI
```yaml
llm:
  provider: azure
  azure:
    api_key: your-key
    endpoint: https://your-resource.openai.azure.com
    deployment: gpt-4
```

### Ollama (locale)
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
Genera minuta sintetica dal testo della trascrizione senza chiamate API.
> Nota: lo Stub è ottimizzato per il template `therapy`. Per altri template le sezioni potrebbero non corrispondere.

---

## Webhook

Vedere [webhook.md](webhook.md) per la documentazione completa.

```yaml
webhook:
  url: https://your-app.example.com/webhook
  timeout: 30s
  mode: push           # "push" (default) o "notify_pull"
  secret: ""           # HMAC secret per notify_pull
  token_ttl: 1h        # TTL token pull
  pull_base_url: ""    # URL pubblico di aftertalk per notify_pull
  delete_on_pull: null # default true per notify_pull
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
  chunk_size_ms: 15000      # dimensione chunk audio per trascrizione (ms)
```

`chunk_size_ms` controlla ogni quanti ms di audio accumulato viene triggerata la trascrizione. Il VAD (Voice Activity Detection) può triggerare prima in caso di silenzio prolungato.

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
    public_ip: ""        # auto-detect se vuoto
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
  max_duration: 2h
  max_participants_per_session: 10

retention:
  transcription_days: 90
  minutes_days: 90
  webhook_events_days: 30
```

---

## Demo Mode

```yaml
demo:
  enabled: false   # NEVER true in produzione — espone l'API key in /demo/config
```

---

## Ottenere il config YAML di default

```bash
./bin/aftertalk --dump-defaults > config.yaml
```
