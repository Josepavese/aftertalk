# Implementation Plan: Aftertalk Core

**Branch**: `001-aftertalk-core` | **Date**: 2026-03-04 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/spec.md`

## Summary

Aftertalk Core is a domain-agnostic AI module for automatically generating end-of-session minutes from WebRTC conversations. The system intercepts audio from WebRTC sessions, transcribes with verified roles, and produces structured minutes. The core is designed to be reusable across different domains (clinical, coaching, business) through configuration and adapters.

**Technical approach**: Modular monolith architecture in Go with a single binary. Internal packages guarantee separation of concerns while retaining the benefits of a single deployment: native performance, in-process communication, simplified operations.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**:
- WebRTC: `pion/webrtc` v4+ (pure Go implementation)
- Audio: `go-audio/opus`, `go-audio/wav` (Opus ↔ PCM conversion)
- HTTP: `chi` (REST API routing)
- STT: Pluggable adapters via HTTP client (Google Cloud Speech-to-Text, AWS Transcribe, Azure Speech)
- LLM: Pluggable adapters via HTTP client (OpenAI GPT-4, Anthropic Claude, Azure OpenAI)
- Database: `modernc.org/sqlite` (pure Go SQLite driver, no CGO)
- Config: `knadh/koanf` (configuration management)
- Logging: `uber-go/zap` (structured logging)

**Storage**: SQLite (embedded database, all data in single file) + In-memory cache (session state, tokens, processing queues)

**Testing**: Go testing package, `testify`, `mockery` (mock generation), `k6` (load testing)

**Target Platform**: Linux containers (Docker/Kubernetes), single binary deployment

**Project Type**: Modular monolith with internal packages (api, bot, ai, core, storage)

**Performance Goals**: <100ms audio acquisition latency (in-process), <5min minutes generation for 1hr session, 200+ concurrent sessions

**Constraints**: No persistent audio storage, append-only transcriptions, privacy-first architecture, single binary deployment

**Scale/Scope**: 200+ concurrent sessions, multi-tenant support via configuration, horizontal scaling with Kubernetes

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Domain-Agnostic Core | ✅ PASS | No domain-specific logic, pluggable providers, abstract roles |
| II. Core/Application Separation | ✅ PASS | Clean internal package boundaries, stable API, no UI dependencies |
| III. Privacy-First | ✅ PASS | No persistent audio, append-only transcriptions, secure tokens, in-memory processing |
| IV. Human-in-the-loop | ✅ PASS | Minutes always editable, no automatic diagnoses, no semantic conclusions |
| V. Design for Reuse | ✅ PASS | Single reusable binary, pluggable providers, declarative config, Go packages for sharing |

**Violations**: None identified

## Project Structure

### Documentation (this feature)

```text
specs/
├── plan.md              # This file
├── research.md          # Phase 0 output (Go technology decisions)
├── data-model.md        # Phase 1 output (database schema)
├── quickstart.md        # Phase 1 output (Go setup guide)
├── contracts/           # Phase 1 output
│   ├── api.yaml              # REST API contract (OpenAPI)
│   └── websocket.yaml        # Bot Recorder WebSocket contract (AsyncAPI)
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
aftertalk/
├── cmd/
│   └── aftertalk/
│       └── main.go              # Single entry point
│
├── internal/
│   ├── api/                     # HTTP REST API layer
│   │   ├── handler/
│   │   │   ├── session.go       # Session endpoints
│   │   │   ├── transcription.go # Transcription endpoints
│   │   │   ├── minutes.go       # Minutes endpoints
│   │   │   └── health.go        # Health checks
│   │   ├── middleware/
│   │   │   ├── auth.go          # JWT validation
│   │   │   ├── logging.go       # Request logging
│   │   │   ├── recovery.go      # Panic recovery
│   │   │   └── cors.go          # CORS handling
│   │   ├── response/
│   │   │   └── json.go          # JSON response helpers
│   │   └── server.go            # HTTP server setup (chi)
│   │
│   ├── bot/                     # WebRTC Bot Recorder
│   │   ├── peer.go              # Pion peer connection management
│   │   ├── audio.go             # Audio processing (Opus → PCM)
│   │   ├── session.go           # Session audio tracking
│   │   ├── timestamp.go         # Server-side timestamping
│   │   ├── auth.go              # JWT token validation
│   │   └── server.go            # WebSocket server
│   │
│   ├── ai/                      # AI Processing Pipeline
│   │   ├── stt/
│   │   │   ├── provider.go      # STT provider interface
│   │   │   ├── google.go        # Google Cloud STT client
│   │   │   ├── aws.go           # AWS Transcribe client
│   │   │   ├── azure.go         # Azure Speech client
│   │   │   └── whisper.go       # Local Whisper (optional)
│   │   ├── llm/
│   │   │   ├── provider.go      # LLM provider interface
│   │   │   ├── openai.go        # OpenAI GPT-4 client
│   │   │   ├── anthropic.go     # Anthropic Claude client
│   │   │   ├── azure.go         # Azure OpenAI client
│   │   │   └── prompts.go       # Prompt templates
│   │   └── pipeline.go          # Pipeline orchestration
│   │
│   ├── core/                    # Business Logic Layer
│   │   ├── session/
│   │   │   ├── service.go       # Session business logic
│   │   │   └── repository.go    # Session data access
│   │   ├── transcription/
│   │   │   ├── service.go       # Transcription logic
│   │   │   └── repository.go    # Transcription data access
│   │   └── minutes/
│   │       ├── service.go       # Minutes logic
│   │       └── repository.go    # Minutes data access
│   │
│   ├── storage/
│   │   ├── sqlite/             # SQLite access
│   │   │   ├── db.go            # Database connection
│   │   │   ├── migrations/      # SQL migrations
│   │   │   │   ├── 001_init.up.sql
│   │   │   │   └── 001_init.down.sql
│   │   │   └── queries/         # SQL queries
│   │   │       ├── session.sql
│   │   │       ├── transcription.sql
│   │   │       └── minutes.sql
│   │   └── cache/               # In-memory cache
│   │       ├── session.go       # Session state cache
│   │       ├── token.go         # Token tracking
│   │       └── queue.go         # Processing queue
│   │
│   └── config/
│       ├── config.go            # Configuration struct
│       └── loader.go            # Config loading (env + file)
│
├── pkg/                         # Public packages (reusable)
│   ├── jwt/
│   │   └── jwt.go               # JWT utilities
│   ├── audio/
│   │   ├── opus.go              # Opus encoding/decoding
│   │   └── pcm.go               # PCM conversion
│   ├── webhook/
│   │   └── client.go            # Webhook notification client
│   └── version/
│       └── version.go           # Build version info
│
├── api/
│   ├── openapi.yaml             # REST API spec
│   └── asyncapi.yaml            # WebSocket API spec
│
├── migrations/
│   ├── 001_init.up.sql
│   └── 001_init.down.sql
│
├── scripts/
│   ├── build.sh                 # Build script
│   └── migrate.sh               # Migration runner
│
├── go.mod
├── go.sum
├── Makefile                     # Build automation
├── Dockerfile                   # Multi-stage build
├── docker-compose.yml           # Local development
├── .golangci.yml                # Linter config
└── README.md
```

**Structure Decision**: Modular monolith in Go with internal packages to ensure separation of concerns. All components (API, Bot, AI, Core) coexist in the same process, communicating via function calls (<1ms latency vs 10-50ms with Redis queues). Simplified deployment: 1 binary = 1 container. Native Go performance handles 200+ concurrent sessions on a single instance.

## Architecture Highlights

### 1. Single Binary Deployment

**Benefits:**
- ✅ Deploy a single file (15MB binary)
- ✅ Startup time: 10ms vs 1-3s (Node.js/Python)
- ✅ Memory: 50MB vs 300MB+ (Node.js/Python)
- ✅ Zero runtime dependencies
- ✅ Cross-compilation: `GOOS=linux GOARCH=amd64 go build`

### 2. In-Process Communication

**Before (Microservices):**
```
Bot → Redis Queue → AI Pipeline
Latency: 10-50ms
Serialization/Deserialization overhead
```

**Now (Monolith):**
```go
// Direct function call
func (b *Bot) OnSessionEnd(sessionID string) {
    transcription, err := b.aiPipeline.Transcribe(sessionID)
    minutes, err := b.aiPipeline.GenerateMinutes(transcription)
    b.minutesRepo.Save(sessionID, minutes)
}
```

**Latency: <1ms** (10-50x faster)

### 3. Concurrent Processing with Goroutines

```go
// Worker pool for parallel processing
func (p *Pipeline) Start(ctx context.Context, workers int) {
    tasks := make(chan Task, workers*2)

    for i := 0; i < workers; i++ {
        go p.worker(ctx, tasks)
    }

    // Submit tasks from session events
    for sessionID := range sessionEvents {
        tasks <- Task{SessionID: sessionID}
    }
}
```

**Benefits:**
- Native goroutines: ~2KB stack vs 1MB+ thread (Java/C++)
- M:N scheduler: Go runtime handles scheduling
- Channel-based communication: idiomatic and thread-safe

### 4. Shared Memory Cache

```go
type Cache struct {
    mu    sync.RWMutex
    items map[string]interface{}
}

// In-process cache, zero network latency
func (c *Cache) Get(key string) interface{} {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.items[key]
}
```

**Benefits:**
- Zero network latency
- No Redis dependency for hot cache data
- Automatic GC management

### 5. Transactional Integrity

```go
// Single database transaction
func (s *Service) ProcessSession(ctx context.Context, sessionID string) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    transcription, err := s.Transcribe(tx, sessionID)
    if err != nil {
        return err
    }

    minutes, err := s.GenerateMinutes(tx, transcription)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

**Benefits:**
- Simplified ACID transactions
- No distributed saga pattern needed
- Guaranteed consistency

## Performance Targets

| Metric | Target | Go Capability |
|--------|--------|---------------|
| Audio acquisition latency | <100ms | ✅ In-process: <1ms |
| Minutes generation (1hr) | <5min | ✅ Parallel processing |
| Concurrent sessions | 200+ | ✅ Goroutines handle 10K+ |
| Memory per session | <1MB | ✅ Go efficient: ~500KB |
| Binary size | <20MB | ✅ ~15MB stripped |
| Startup time | <100ms | ✅ ~10ms |

## Cost Comparison

| Metric | Go Monolith | Node.js/Python Microservices |
|--------|-------------|------------------------------|
| Containers | 1 | 3 |
| Memory baseline | 50MB | 300MB+ |
| CPU per session | ~0.5% | ~2% |
| Cloud cost (100 sessions) | $200/mo | $800/mo |
| Savings | **75%** | - |

## Complexity Tracking

> No violations to justify — modular monolith respects all constitutional principles with greater simplicity.

**Architectural justification:**
- Modular monolith ≠ spaghetti code: internal packages enforce boundaries
- Native Go performance eliminates the need for microservices for scaling
- Operational simplicity > over-engineering for 200 concurrent sessions
- 75% cost reduction without compromising quality
