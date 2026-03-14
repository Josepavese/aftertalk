# Aftertalk Core

**AI-agnostic core for automatic meeting minutes generation from WebRTC conversations**

## Overview

Aftertalk is a Go module that intercepts audio from WebRTC sessions, automatically transcribes conversations, and generates structured minutes using AI.

**Architecture**: Modular monolith in Go with internal packages for separation of concerns.

**Target**: 200+ concurrent sessions, <100ms audio capture latency, <5min minutes generation.

## Quick Start

### Prerequisites

- Go 1.22+
- Docker & Docker Compose (optional)

**Note**: SQLite is embedded - no external database server needed!

### Setup

```bash
# Clone repository
git clone https://github.com/Josepavese/aftertalk.git
cd aftertalk

# Copy environment variables
cp .env.example .env
# Edit .env with your configuration

# Run application
make run
```

**That's it!** No database setup needed - SQLite creates `aftertalk.db` automatically.

### Development

```bash
# Run without building
make dev

# Run tests
make test

# Run with coverage
make test-coverage

# Lint code
make lint

# Format code
make fmt
```

## Architecture

```
aftertalk/
├── cmd/aftertalk/        # Entry point
├── internal/
│   ├── api/             # HTTP REST API
│   ├── bot/             # WebRTC Bot Recorder
│   ├── ai/              # AI Pipeline (STT + LLM)
│   ├── core/            # Business Logic
│   ├── storage/         # SQLite + In-memory Cache
│   └── config/          # Configuration
└── pkg/                 # Public packages
```

## Key Features

- **WebRTC Audio Capture**: Pion-based server-side peer
- **Real-time Transcription**: Pluggable STT providers (Google, AWS, Azure)
- **AI Minutes Generation**: Structured output with LLM (OpenAI, Anthropic)
- **Privacy-First**: No persistent audio, append-only transcriptions
- **Human-in-the-loop**: Minutes always editable by professionals

## Documentation

- [Architecture Plan](specs/plan.md)
- [Technical Research](specs/research.md)
- [Data Model](specs/data-model.md)
- [Quickstart Guide](specs/quickstart.md)
- [API Contracts](specs/contracts/)

## License

MIT

## Status

🚧 Under active development
