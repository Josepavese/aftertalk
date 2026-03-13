# Improvement: Installer — SSOT & PAL Compliance

## Devil's Advocate Verdict

**The claim "simple, fully configurable installer written with SSOT/PAL approach" is PARTIALLY true.**

The system is functional and impressive in scope, but has structural gaps relative to the declared principles.

---

## Identified Gaps

### 1. SSOT Violation — Hardcoded Values in Go Code

**Problem**: `config.yaml` is the declared SSOT, but hardcoded values are scattered in the code that should come from the config.

| Hardcoded Value | File | Line | Should Be In |
|---|---|---|---|
| `stun:stun.l.google.com:19302` | `internal/bot/webrtc/peer.go:37` | 37 | `config.yaml → webrtc.stun_servers` |
| WebSocket timeout `30s` | `internal/bot/webrtc/signaling.go` | various | `config.yaml → webrtc.timeout` |
| Default chunk size `15s` | `internal/core/session/service.go` | various | already in config but not always respected |
| `chunkSizeMs = 15000` | `internal/bot/webrtc/peer.go` | constant | `config.yaml → processing.chunk_size_ms` |
| `buffer = 100` (transcription chan) | `internal/core/session/service.go` | various | `config.yaml → processing.buffer_size` |
| `MaxRetries: 3` LLM | `internal/core/minutes/service.go:39` | 39 | `config.yaml → llm.retry.max_retries` |
| Minutes generation timeout | main.go | constants | `config.yaml → processing.timeouts` |

**Required Fix**:
```yaml
# config.yaml — add section:
webrtc:
  stun_servers:
    - stun:stun.l.google.com:19302
  websocket_timeout: 30s
  ice_gathering_timeout: 10s

processing:
  chunk_size_ms: 15000
  transcription_buffer_size: 100
  llm_retry:
    max_retries: 3
    initial_backoff: 1s
    max_backoff: 10s
```

---

### 2. PAL Violation — Non-Modular Installer

**Problem**: `install.sh` is a monolithic 413-line file mixing responsibilities:
- Platform detection (partially extracted to `_platform.sh`, but not completely)
- Go installation logic
- Python/pip management
- Ollama management
- Binary build
- Config generation
- CLI wrapper generation

**PAL Violation**: The "Logic Layer" (what to install) is mixed with the "Provider Layer" (how to install it on each OS).

**Required Fix**: Separate into distinct modules:

```
scripts/
├── install.sh              # Orchestrator (Logic Layer)
├── _platform.sh            # Platform detection (Middleware)
├── providers/
│   ├── _go.sh              # Go installation provider
│   ├── _python.sh          # Python/pip provider
│   ├── _ollama.sh          # Ollama provider
│   └── _whisper.sh         # Whisper server provider
└── steps/
    ├── _config.sh          # Config generation step
    ├── _binary.sh          # Build step
    └── _cli.sh             # CLI wrapper generation step
```

---

### 3. Missing SSOT — Installer vs Code Versioning

**Problem**: The version hardcoded in the installer (`Aftertalk Installer v1.0`) is not synchronized with any version file in the project.

```bash
# install.sh:85
echo "  ║     Aftertalk Installer v1.0      ║"
```

There is no `VERSION` file, no `version.go`, and no field in `go.mod` serving as SSOT for the version. The version exists only in the banner string.

**Required Fix**:
```
# Create /version.txt (or read from git tag)
1.0.0

# install.sh reads from this file or git tag:
VERSION=$(cat "$AFTERTALK_SRC/version.txt" 2>/dev/null || git -C "$AFTERTALK_SRC" describe --tags 2>/dev/null || echo "dev")
```

---

### 4. Missing SSOT — Duplicated Defaults Between Installer and Go Config Struct

**Problem**: Default values exist in **two places**:
1. `scripts/install.sh` (generates `config.yaml` with hardcoded defaults)
2. `internal/config/config.go` (Go struct with `default:...` tags via koanf)

If a default changes in the Go code, the installer will not know and will generate a `config.yaml` with the old value.

**Concrete example**:
```bash
# install.sh:230
processing:
  chunkSizeMs: 15000   # hardcoded in installer
```
```go
// config.go — if it ever changes to 30000:
type ProcessingConfig struct {
    ChunkSizeMs int `koanf:"chunkSizeMs" default:"30000"`
}
```

**Required Fix**: The installer should generate the config with defaults extracted from the binary itself:
```bash
# After building the binary:
"$AFTERTALK_BIN/aftertalk-server" --dump-config-defaults > "$CONFIG_FILE"
```
This requires a `--dump-config-defaults` flag in the Go server that outputs YAML with all default values.

---

### 5. Incomplete Windows Installer (Unbalanced PAL)

**Problem**: `install.ps1` exists but lacks functional parity with `install.sh`:
- Does not handle `aftertalk update`
- Process management (PID files) uses different approaches between Unix and Windows
- Not tested in CI
- Missing `install.bat` as entry point (end-user Windows users don't use PowerShell directly)

**Required Fix**:
- `install.bat` entry point that launches `install.ps1` correctly
- Full CLI command parity (`start`, `stop`, `status`, `update`, `logs`)
- CI tests for Windows (GitHub Actions `windows-latest`)

---

### 6. No Installer Integrity Verification

**Problem**: The `curl -fsSL ... | bash` pattern is insecure without signature verification.

```bash
# Current pattern — vulnerable to MITM:
curl -fsSL https://raw.githubusercontent.com/Josepavese/aftertalk/master/scripts/install.sh | bash
```

**Required Fix**:
```bash
# Secure pattern with SHA256 verification:
curl -fsSL https://releases.aftertalk.io/install.sh -o install.sh
curl -fsSL https://releases.aftertalk.io/install.sh.sha256 -o install.sh.sha256
sha256sum --check install.sh.sha256
bash install.sh
```

---

### 7. aftertalk CLI — No Health Recovery Mechanism

**Problem**: The `aftertalk start` command starts processes but does not monitor them. If whisper_server crashes after startup, nobody detects it.

**Required Fix**: Process supervision system:
- Integration with `systemd` (Linux) or `launchd` (macOS) for auto-restart
- On Windows: Windows Service via NSSM or Task Scheduler
- Or: supervisord-style watchdog in the CLI wrapper

---

## Intervention Priority

| # | Gap | Impact | Effort | Priority |
|---|-----|--------|--------|----------|
| 1 | STUN hardcoded in Go | Medium | Low | **High** |
| 4 | Duplicated installer/Go defaults | High | Medium | **High** |
| 2 | Non-modular installer | Medium | Medium | **Medium** |
| 3 | Missing versioning | Low | Low | **Medium** |
| 7 | No health recovery | High | High | **Medium** |
| 5 | Incomplete Windows | Medium | High | **Low** |
| 6 | Installer security | Low | Medium | **Low** |

---

## Implementation Steps

### Step 1 — Externalize STUN to config (1-2h)

1. Add `WebRTCConfig` struct to `internal/config/config.go`
2. Add `webrtc:` section to config.yaml template in installer
3. Read STUN servers in `peer.go` from `cfg.WebRTC.STUNServers`
4. Test: verify WebRTC works with custom STUN servers

### Step 2 — Add `--dump-config-defaults` (2-3h)

1. Add `--dump-defaults` flag to main.go
2. When active: print YAML with all default values from koanf and exit
3. Update installer: after build, run `aftertalk-server --dump-defaults > config.yaml`
4. Remove config heredoc block from install.sh

### Step 3 — VERSION file (30min)

1. Create `/version.txt` with current value
2. Read in install.sh: `VERSION=$(cat "$AFTERTALK_SRC/version.txt")`
3. Read in main.go: `go:embed version.txt`
4. Expose in `/v1/health` response: `{"status":"ok","version":"1.0.0"}`

### Step 4 — Modularize install.sh (3-4h)

1. Extract each functional block into `scripts/providers/_*.sh`
2. Each provider has: `check_<tool>()`, `install_<tool>()`, `version_<tool>()`
3. `install.sh` becomes a pure orchestrator that sources the providers
4. Add shell unit tests (bats framework)
