# Agent Context: Aftertalk Core Implementation

## Project Overview

**Project**: Aftertalk Core - Go Monolith  
**Architecture**: Modular monolith with internal packages  
**Tech Stack**: Go 1.22+, Pion WebRTC, Chi HTTP router, pgx PostgreSQL, Zap logging

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
│   ├── storage/                  # Database + Redis
│   └── config/                   # Configuration
└── pkg/                        # Public packages
```

## Database Schema

See `specs/001-aftertalk-core/data-model.md` for complete schema.

Key entities:
- Session
- Participant  
- AudioStream
- Transcription
- Minutes

## API Contracts

- REST API: `specs/001-aftertalk-core/contracts/api.yaml`
- WebSocket: `specs/001-aftertalk-core/contracts/websocket.yaml`
- Internal Interfaces: `specs/001-aftertalk-core/contracts/internal-interfaces.md`

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
