# Aftertalk Core

**Core AI agnostico per generazione automatica di minute di conversazioni WebRTC**

## Overview

Aftertalk è un modulo Go che intercetta audio da sessioni WebRTC, trascrive automaticamente le conversazioni e genera minute strutturate usando AI.

**Architettura**: Monolite modulare in Go con internal packages per separazione delle responsabilità.

**Target**: 200+ sessioni concorrenti, <100ms latenza acquisizione audio, <5min generazione minuta.

## Quick Start

### Prerequisites

- Go 1.22+
- PostgreSQL 15+
- Redis 7+ (optional)
- Docker & Docker Compose

### Setup

```bash
# Clone repository
git clone https://github.com/flowup/aftertalk.git
cd aftertalk

# Copy environment variables
cp .env.example .env
# Edit .env with your configuration

# Start infrastructure
docker-compose up -d postgres redis

# Run migrations
psql $DATABASE_URL < migrations/001_init.up.sql

# Run application
make run
```

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
│   ├── storage/         # Database + Redis
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

- [Architecture Plan](specs/001-aftertalk-core/plan.md)
- [Technical Research](specs/001-aftertalk-core/research.md)
- [Data Model](specs/001-aftertalk-core/data-model.md)
- [Quickstart Guide](specs/001-aftertalk-core/quickstart.md)
- [API Contracts](specs/001-aftertalk-core/contracts/)

## License

MIT

## Status

🚧 Under active development
