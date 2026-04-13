# Architecture

Aftertalk is a modular Go monolith. No microservices, no external dependencies beyond SQLite.

## Directory Structure

```
aftertalk/
├── cmd/aftertalk/          # Entry point: DI wiring, DB migrations, server start
│   └── main.go
├── internal/
│   ├── api/                # HTTP layer (chi router)
│   │   ├── server.go       # NewServerWithDeps(), route registration
│   │   ├── bot_stub.go     # WebRTC BotServer (WebSocket + signaling bridge)
│   │   ├── handler/        # Request handlers (session, transcription, minutes, health)
│   │   └── middleware/     # CORS, rate limit, logging, recovery, API key auth
│   ├── bot/                # WebRTC internals
│   │   └── webrtc/
│   │       ├── peer.go     # Pion PeerConnection, ICE setup, audio track handling
│   │       ├── signaling.go # WebSocket signaling server (offer/answer/ICE candidates)
│   │       └── ice_provider.go  # ICE PAL: static / embedded-TURN / Twilio / Xirsys / Metered
│   ├── core/               # Business logic (pure Go, no HTTP/DB imports)
│   │   ├── session/        # Session entity, service, repository interface
│   │   ├── transcription/  # Transcription entity, service, repository interface
│   │   └── minutes/        # Minutes entity, service, repository interface
│   ├── ai/
│   │   ├── stt/            # STT PAL: provider interface, factory, retry, providers
│   │   └── llm/            # LLM PAL: provider interface, factory, providers, prompts
│   ├── storage/
│   │   ├── sqlite/         # SQLite adapter: DB init, RunInTx, repository implementations
│   │   └── cache/          # In-memory caches: sessions, tokens, audio buffers
│   ├── config/             # Config struct, koanf loader, template defaults
│   ├── logging/            # Zap logger wrapper
│   └── metrics/            # Prometheus metrics
├── pkg/
│   ├── jwt/                # JWTManager: Generate, Validate (golang-jwt)
│   ├── audio/              # Opus decoder, PCM converter, VAD
│   └── webhook/            # HTTP client for push/notify_pull delivery
├── sdk/                    # TypeScript SDK (@aftertalk/sdk)
├── cmd/test-ui/            # Embedded demo UI (static HTML)
├── docs/wiki/              # This wiki
├── specs/contracts/        # OpenAPI spec (api.yaml)
└── e2e/                    # E2E tests (starts embedded server, calls HTTP)
```

---

## Key Design Principles

### PAL (Platform Abstraction Layer)

Business logic consumes only interfaces. Providers are plugged in at startup via factory functions. This pattern applies to:

- **STT**: `stt.STTProvider` interface → `stt.NewProvider(cfg)` → Google / AWS / Azure / WhisperLocal / Stub
- **LLM**: `llm.LLMProvider` interface → `llm.NewProvider(cfg)` → OpenAI / Anthropic / Azure / Ollama / Stub
- **ICE**: `webrtc.ICEProvider` interface → `webrtc.NewICEProvider(cfg)` → Static / EmbeddedTURN / Twilio / Xirsys / Metered
- **Storage**: repository interfaces → SQLite implementations

### SSOT (Single Source of Truth)

- Templates defined once in `config.go:DefaultTemplates()` + optional YAML override
- DB schema defined once in `main.go:runMigrations()`
- API key defined once in config, checked by a single middleware

---

## Session Lifecycle

```
POST /v1/sessions
  └─ session.Service.CreateSession()
       ├─ Persist to sessions + participants tables
       ├─ Generate JWT tokens (one per participant)
       └─ Cache in SessionCache + TokenCache

Client connects via WebSocket → /signaling?token=eyJ...
  └─ SignalingServer.HandleWebSocket()
       └─ WebRTCManager.CreatePeer()
            └─ Pion PeerConnection
                 ├─ SetLocalDescription → send answer via WS
                 └─ OnTrack → handleAudioTrack() → ReadRTP loop

RTP audio received
  └─ session.Service.ProcessAudioChunk()
       ├─ Buffer in AudioBufferCache
       └─ Every 15s (`processing.chunk_size_ms`): flush → transcribeCh (buffered chan 100)

Transcription worker
  └─ processTranscriptionQueue()
       ├─ Opus decode → PCM 16kHz (pkg/audio)
       ├─ stt.Provider.Transcribe()
       └─ Persist transcription to DB

POST /v1/sessions/{id}/end
  └─ session.Service.EndSession()
       ├─ status → "ended"
       ├─ processRemainingAudio() — drains buffer
       └─ generateMinutesForSession()
            ├─ Wait for in-flight transcription writes to complete
            ├─ Read ordered transcriptions from DB
            ├─ Split transcript into bounded batches
            ├─ llm.Provider.Generate() on compact minutes state + next batch
            ├─ Normalize summary / phases / citations after each pass
            ├─ Optional final compaction pass
            ├─ ParseMinutesDynamic() → Minutes{Summary, Sections, Citations}
            ├─ Persist to minutes table
            ├─ status → "completed"
            └─ webhook.Client.Send() / SendNotification()

Inactivity timeout: 10 minutes of silence → auto-EndSession()
Session reaper: sweep every 5 min → EndSession() for sessions older than max_duration

Session status transitions:
  active → processing → completed   (normal path)
  active → processing → error       (LLM/transcription failure; session.Fail())
  processing → completed/error      (recovered at boot by RecoverProcessingSessions)
```

---

## Audio Pipeline Detail

```
WebRTC RTP packet (Opus encoded)
  │
  ▼
peer.go: ReadRTP() → raw Opus payload
  │
  ▼
AudioBufferCache.Append() — accumulates payloads
  │
  ▼ (triggered by timer or VAD silence detection)
pkg/audio.OpusDecoder.Decode() → PCM float32 @ 48kHz
  │
  ▼
pkg/audio.PCMConverter.ConvertToInt16_16k() → PCM int16 LE @ 16kHz
  │
  ▼
stt.Provider.Transcribe(pcm16kBytes) → text
  │
  ▼
transcription.Service.CreateTranscription()
```

---

## Database Schema

All tables created inline in `main.go:runMigrations()`.

| Table | Purpose |
|---|---|
| `sessions` | Session metadata, status, template_id |
| `participants` | JWT tokens, roles, user IDs |
| `audio_streams` | Per-participant audio stream state |
| `transcriptions` | Text segments with timestamps and role |
| `minutes` | Generated minutes JSON blob (`summary`, `sections`, `citations`) + template_id |
| `minutes_history` | Previous versions of minutes (for auditing) |
| `webhook_events` | Delivery attempts log |
| `processing_queue` | Pending STT/LLM jobs |
| `retrieval_tokens` | Single-use tokens for notify_pull mode |

---

## Dependency Injection

All wiring happens in `cmd/aftertalk/main.go`. There is no DI framework — everything is explicit:

```
Config → Providers (STT, LLM, ICE) → Services (session, transcription, minutes)
  └─ Adapters (TranscriptionAdapter, MinutesAdapter) bridge service interfaces
  └─ Handlers receive services via constructor injection
  └─ Server receives handlers + botServer
```

---

## Concurrency Model

- Each WebRTC peer runs its RTP read loop in a goroutine
- Transcription queue: single goroutine reading from a buffered `chan` (size 100)
- Minutes generation: goroutine spawned per session end; internally reduced in bounded LLM batches
- In-memory caches use `sync.RWMutex`
- SQLite runs in WAL mode — concurrent reads with serialized writes

---

## What's Not in the Architecture

- No message queue (Kafka, RabbitMQ) — everything in-process via channels
- No external cache (Redis) — all state in SQLite + in-memory Go maps
- No separate worker process — processing is embedded in the server binary
- No audio files persisted — only decoded PCM bytes pass through memory to STT
