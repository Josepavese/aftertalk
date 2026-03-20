# Aftertalk Core

**AI-agnostic core for automatic meeting minutes generation from WebRTC conversations**

## Overview

Aftertalk is a Go module that intercepts audio from WebRTC sessions, automatically transcribes conversations, and generates structured minutes using AI.

**Architecture**: Modular monolith in Go with internal packages for separation of concerns.

**Target**: 200+ concurrent sessions, <100ms audio capture latency, <5min minutes generation.

## Install

Pre-built binaries for Linux, macOS, and Windows are published automatically on every release.

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.sh | bash
```

Modes (default: `local-ai` — installs Whisper + Ollama locally):

```bash
# Cloud STT/LLM providers (configure keys after install)
curl -fsSL .../install.sh | bash -s -- --mode=cloud

# No AI (stub providers, useful for testing)
curl -fsSL .../install.sh | bash -s -- --mode=offline
```

### Windows

Run in PowerShell **as Administrator**:

```powershell
irm https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.ps1 | iex
```

### Manual binary download

Download the binary for your platform from [Releases](https://github.com/Josepavese/aftertalk/releases/latest):

| Platform | File |
|---|---|
| Linux x86-64 | `aftertalk-linux-amd64` |
| Linux ARM64  | `aftertalk-linux-arm64` |
| macOS x86-64 | `aftertalk-darwin-amd64` |
| macOS Apple Silicon | `aftertalk-darwin-arm64` |
| Windows x86-64 | `aftertalk-windows-amd64.exe` |
| Windows ARM64  | `aftertalk-windows-arm64.exe` |

### Environment overrides

| Variable | Default | Description |
|---|---|---|
| `AFTERTALK_HOME` | `~/.aftertalk` | Install directory |
| `AFTERTALK_RELEASE` | `latest` | Release to install (`latest` or `edge` for master builds) |
| `WHISPER_MODEL` | `base` | Whisper model size |
| `SKIP_WHISPER` | — | Set to `1` to skip Whisper setup |
| `SKIP_OLLAMA` | — | Set to `1` to skip Ollama setup |

## Build from source

```bash
git clone https://github.com/Josepavese/aftertalk.git
cd aftertalk
go build -o bin/aftertalk-server ./cmd/aftertalk
```

## Development

```bash
# Run without building
make dev

# Run tests
make test

# Run with coverage
make test-coverage

# Lint
make lint
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
- **Real-time Transcription**: Pluggable STT providers (Google, AWS, Azure, Whisper)
- **AI Minutes Generation**: Structured output with LLM (OpenAI, Anthropic, Ollama)
- **Privacy-First**: No persistent audio, append-only transcriptions
- **Human-in-the-loop**: Minutes always editable by professionals

## TLS / HTTPS

By default Aftertalk serves plain HTTP, suitable when behind a reverse proxy (Apache, nginx) that handles TLS termination.

To run with native HTTPS/WSS, add to `aftertalk.yaml`:

```yaml
tls:
  cert_file: /path/to/cert.pem
  key_file:  /path/to/key.pem
```

If the files are configured but missing at startup, the server exits with an explicit error — it never silently falls back to plain HTTP.

## SDKs

| SDK | Language | Use case |
|-----|----------|----------|
| [`@aftertalk/sdk`](sdk/ts/) | TypeScript / JS | Browser frontend — WebRTC audio streaming, minutes polling |
| [`aftertalk/aftertalk-php`](sdk/php/) | PHP 8.1+ | Server-side backend — session management, webhook verification |

See the [Integration Guide](docs/wiki/integration-guide.md) for the canonical pattern:
PHP backend holds the API key; browser receives only a short-lived JWT room token.

## Documentation

- [Wiki](docs/wiki/)
- [Integration Guide](docs/wiki/integration-guide.md)
- [Architecture Plan](specs/plan.md)
- [API Contracts](specs/contracts/)

## License

MIT
