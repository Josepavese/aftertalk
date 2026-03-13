# Agent Context: Aftertalk Core Implementation

## Project Overview

**Project**: Aftertalk Core - Go Monolith  
**Architecture**: Modular monolith with internal packages  
**Tech Stack**: Go 1.25+, Pion WebRTC v4.2.9, Chi HTTP router v5, SQLite (modernc.org/sqlite v1.46.1), Zap logging v1.27.1

## Implementation Status

- ✅ Constitution: v1.0.0 - Core Agnostico, Privacy-First, Human-in-the-loop
- ✅ Specification: 4 User Stories (P1-P4)
- ✅ Plan: Go architecture defined
- ✅ Tasks: 89 tasks organized by phase

## Current Work

Working on User Story **[STORY_LABEL]**: [STORY_DESCRIPTION]

## Task Progress

[PROGRESS_TRACKING]

## Key Files and Structure

```
aftertalk/
├── cmd/aftertalk/main.go           # Entry point
├── internal/
│   ├── api/                      # HTTP REST API
│   ├── bot/                      # WebRTC Bot Recorder
│   ├── ai/                       # AI Pipeline (STT + LLM)
│   ├── core/                     # Business Logic
│   ├── storage/                  # SQLite + In-memory Cache
│   └── config/                   # Configuration
└── pkg/                        # Public packages
```

## Database Schema

See `specs/data-model.md` for complete schema.

**Database**: SQLite (embedded, single file)
- All data stored in `aftertalk.db`
- Zero external dependencies
- WAL mode for concurrent read/write
- In-memory cache for session state, tokens, and processing queues

Key entities:
- Session
- Participant  
- AudioStream
- Transcription
- Minutes

## API Contracts

- REST API: `specs/contracts/api.yaml`
- WebSocket: `specs/contracts/websocket.yaml`
- Internal Interfaces: `specs/contracts/internal-interfaces.md`

## Dependencies Policy

**CRITICAL**: Always use the **latest stable versions** of all dependencies.

- ✅ **No legacy versions**: Always update to latest releases
- ✅ **Security first**: Immediate updates for security patches
- ✅ **No CGO**: Prefer pure Go implementations (SQLite, WebRTC)
- ✅ **Minimal dependencies**: Only essential packages
- ✅ **Active maintenance**: All dependencies must be actively maintained

### Key Dependencies (Latest Versions)

- **pion/webrtc v4.2.9** - WebRTC in pure Go (latest stable)
- **modernc.org/sqlite v1.46.1** - Pure Go SQLite driver
- **go-chi/chi v5** - Lightweight HTTP router
- **uber-go/zap v1.27.1** - Structured logging
- **golang-jwt/jwt v5.3.1** - JWT implementation
- **knadh/koanf v2.3.2** - Configuration management

**Update Command**: `go get -u all && go mod tidy`

See `docs/DEPENDENCIES.md` for complete dependency list.

---

## Development Guidelines

### Go Conventions

- Use `internal/` for private packages
- Use `pkg/` for reusable public packages
- Follow Go naming conventions (camelCase for exported names)
- Use context for cancellation and timeout
- Implement interfaces for testability

### Error Handling

- Use custom error types with `fmt.Errorf` and `%w` for wrapping
- Implement proper HTTP status codes in API handlers
- Log errors with structured logging (zap)

### Configuration

- Use koanf for configuration loading
- Support environment variables
- Support YAML config file
- Provide sensible defaults

### Testing Strategy

- Unit tests for business logic
- Integration tests for database operations
- Use testify for assertions
- Use mockery for mock generation (if needed)

## Next Steps After This Phase

1. Run tests to validate implementation
2. Update documentation if needed
3. Move to next user story phase
4. Or run `/speckit.analyze` for consistency check

## Notes

- All file paths are relative to repository root
- Use context propagation for request scoping
- Implement graceful shutdown
- Follow Constitution principles (Privacy-First, Human-in-the-loop)
