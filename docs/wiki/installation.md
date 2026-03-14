# Installation

## Requirements

- Linux / macOS (Windows via WSL)
- Go 1.22+ (only for building from source)
- No other requirements — SQLite is embedded, TURN is an optional embedded feature

## Automatic installer

```bash
curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.sh | bash
```

The installer:
1. Detects Go, Python, Whisper (if present)
2. Builds the binary to `bin/aftertalk`
3. Creates `.env` from `.env.example` if it does not exist

To inspect it before running:

```bash
curl -fsSL .../install.sh | less
```

## Manual build

```bash
git clone https://github.com/Josepavese/aftertalk
cd aftertalk
go build -o bin/aftertalk ./cmd/aftertalk
```

## First run

```bash
cp .env.example .env
# Edit at minimum: AFTERTALK_JWT_SECRET, AFTERTALK_API_KEY, AFTERTALK_LLM_PROVIDER
./bin/aftertalk
```

On first startup the server:
- Creates the SQLite database at `./aftertalk.db` (configurable path)
- Runs inline migrations (no separate SQL files)
- Listens on `0.0.0.0:8080`

Verify:
```bash
curl http://localhost:8080/v1/health
# → {"status":"ok"}
```

## Docker

```bash
docker build -t aftertalk:latest .
docker-compose up -d
```

The included `docker-compose.yml` mounts a volume for the database.

## Update

```bash
git pull && go build -o bin/aftertalk ./cmd/aftertalk
# Migrations run automatically at startup
```
