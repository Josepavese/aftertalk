# Installation

Pre-built binaries for Linux, macOS, and Windows are published automatically to
[GitHub Releases](https://github.com/Josepavese/aftertalk/releases) on every push to `master`
(rolling `edge` pre-release) and on every version tag (stable release).

No Go installation is required to install Aftertalk.

---

## Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.sh | bash
```

The installer:
1. Downloads the pre-built binary for your OS and architecture
2. Sets up Whisper (faster-whisper via pip) and Ollama for local AI
3. Creates `~/.aftertalk/{bin,config,data,logs,models}` directory structure
4. Generates a `config.yaml` with random API key and JWT secret
5. Installs an `aftertalk` CLI command at `/usr/local/bin/aftertalk`

### Installation modes

| Mode | Command | Description |
|---|---|---|
| `local-ai` *(default)* | *(no flag)* | Installs Whisper + Ollama locally |
| `cloud` | `--mode=cloud` | Skips local AI; configure STT/LLM API keys in config |
| `offline` | `--mode=offline` | No AI — uses stub providers (for testing only) |

```bash
# Cloud mode example
curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.sh \
  | bash -s -- --mode=cloud
```

### Environment overrides

```bash
AFTERTALK_HOME=/opt/aftertalk \
AFTERTALK_RELEASE=edge \
WHISPER_MODEL=small \
  curl -fsSL .../install.sh | bash
```

| Variable | Default | Description |
|---|---|---|
| `AFTERTALK_HOME` | `~/.aftertalk` | Install root directory |
| `AFTERTALK_RELEASE` | `latest` | Release tag (`latest` or `edge`) |
| `WHISPER_MODEL` | `base` | Whisper model: `tiny` `base` `small` `medium` `large` |
| `WHISPER_LANGUAGE` | *(auto)* | Force transcription language, e.g. `en` `it` |
| `OLLAMA_MODEL` | `qwen3:4b` | Ollama model to pull |
| `SKIP_WHISPER` | — | Set to `1` to skip Whisper setup |
| `SKIP_OLLAMA` | — | Set to `1` to skip Ollama setup |

---

## Windows

Run in **PowerShell as Administrator**:

```powershell
irm https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.ps1 | iex
```

The installer:
1. Downloads `aftertalk-windows-amd64.exe` (or `arm64`) from GitHub Releases
2. Installs Python 3 if not present (via winget/choco/scoop)
3. Sets up faster-whisper and Ollama
4. Creates `%LOCALAPPDATA%\aftertalk\` directory structure
5. Installs `aftertalk.bat` / `aftertalk.ps1` CLI wrappers and adds them to the user PATH

### Environment overrides (PowerShell)

```powershell
$env:AFTERTALK_RELEASE = "edge"
$env:WHISPER_MODEL     = "small"
$env:SKIP_OLLAMA       = "1"
irm https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.ps1 | iex
```

---

## Manual binary download

Download the binary for your platform directly from
[Releases → latest](https://github.com/Josepavese/aftertalk/releases/latest):

| Platform | File |
|---|---|
| Linux x86-64 | `aftertalk-linux-amd64` |
| Linux ARM64 (Raspberry Pi, AWS Graviton) | `aftertalk-linux-arm64` |
| macOS Intel | `aftertalk-darwin-amd64` |
| macOS Apple Silicon (M1/M2/M3) | `aftertalk-darwin-arm64` |
| Windows x86-64 | `aftertalk-windows-amd64.exe` |
| Windows ARM64 | `aftertalk-windows-arm64.exe` |

```bash
# Example: Linux x86-64
curl -fsSL https://github.com/Josepavese/aftertalk/releases/latest/download/aftertalk-linux-amd64 \
  -o aftertalk-server
chmod +x aftertalk-server
cp .env.example .env   # edit as needed
./aftertalk-server
```

---

## Build from source

Requires Go 1.22+:

```bash
git clone https://github.com/Josepavese/aftertalk
cd aftertalk
go build -o bin/aftertalk-server ./cmd/aftertalk
```

---

## First run

```bash
cp .env.example .env
# Edit at minimum: JWT_SECRET, API_KEY, LLM_PROVIDER
./bin/aftertalk-server
```

On first startup the server:
- Creates the SQLite database at `./aftertalk.db` (configurable via `DATABASE_PATH`)
- Runs inline migrations automatically (no external SQL files)
- Listens on `0.0.0.0:8080`

Verify:
```bash
curl http://localhost:8080/v1/health
# → {"status":"ok"}
```

---

## Docker

```bash
docker build -t aftertalk:latest .
docker-compose up -d
```

The included `docker-compose.yml` mounts a volume for the database.

---

## Update

```bash
# via CLI (Linux/macOS/Windows)
aftertalk update

# or manual
curl -fsSL https://github.com/Josepavese/aftertalk/releases/latest/download/aftertalk-linux-amd64 \
  -o ~/.aftertalk/bin/aftertalk-server
# Migrations run automatically at next startup
```
