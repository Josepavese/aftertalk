# Improvement: Fullstack Self-Contained

## Devil's Advocate Verdict

**The claim "install and use, nothing external" is FALSE for most real use cases.**

The Go core is self-contained. But the working transcription + minutes pipeline mandatorily requires external services. The claim is only true in "stub" mode (a demo that is unusable in production).

---

## Identified Gaps

### 1. Cloud STT Providers — Stubs, Not Real Implementations

**Critical problem**: Google, AWS and Azure STT are **stubs with placeholders**, not real implementations.

```go
// internal/ai/stt/providers.go — Google "provider"
func (p *GoogleSTTProvider) Transcribe(...) (*TranscriptionResult, error) {
    segment := &TranscriptionSegment{
        Text: "[Transcription placeholder - Google STT integration required]",
        // ↑ PLACEHOLDER, not a real API call
    }
}
```

**Impact**: If a user configures `stt.provider: google` and provides credentials, they will receive placeholder text instead of real transcriptions. There is no visible warning that it is a placeholder.

**Only whisper-local is functional** (via HTTP to the Python server). But whisper-local requires Python + faster-whisper as an external dependency.

**Required Fix**:
Implement real HTTP calls for each provider:

```go
// Google Speech-to-Text REST API
POST https://speech.googleapis.com/v1/speech:recognize
Authorization: Bearer {access_token}
Content-Type: application/json
{
  "config": {"encoding": "LINEAR16", "sampleRateHertz": 16000, "languageCode": "en-US"},
  "audio": {"content": "{base64_audio}"}
}
```

```go
// AWS Transcribe — Streaming API or StartTranscriptionJob
// Azure Speech Services — Batch Transcription API
```

---

### 2. Hardcoded STUN Server — Google (Privacy + Availability)

**Problem**: WebRTC exclusively uses `stun:stun.l.google.com:19302` hardcoded.

```go
// internal/bot/webrtc/peer.go:37
{URLs: []string{"stun:stun.l.google.com:19302"}},
```

**Implications**:
1. **Privacy**: Every WebRTC connection reveals the server's public IP to Google
2. **Reliability**: If Google STUN is unreachable (corporate firewalls), WebRTC fails
3. **Private environments**: Air-gapped or corporate networks cannot use public STUNs
4. **Missing TURN**: With symmetric NAT (common in enterprise), without a TURN server connections fail completely

**Required Fix**:
```yaml
# config.yaml
webrtc:
  ice_servers:
    - urls: ["stun:stun.l.google.com:19302"]  # default, configurable
  turn_servers: []  # optional
  # turn_servers:
  #   - urls: ["turn:turn.example.com:3478"]
  #     username: "user"
  #     credential: "password"
```

```go
// peer.go — read from config
iceServers := make([]webrtc.ICEServer, 0, len(cfg.WebRTC.ICEServers))
for _, s := range cfg.WebRTC.ICEServers {
    iceServers = append(iceServers, webrtc.ICEServer{URLs: s.URLs, ...})
}
```

---

### 3. Whisper-Local — Non-Eliminable Python Dependency

**Problem**: The default STT pipeline requires Python 3.9+, faster-whisper, ffmpeg as runtime dependencies external to the Go binary.

```
aftertalk-server (Go, self-contained)
    ↓ HTTP POST localhost:9001
whisper_server.py (Python, external dependency)
    ↓ uses
faster-whisper (pip package)
    ↓ uses
ffmpeg (system binary)
```

**Not "nothing external"**: the user must have working Python. If Python or pip update and break compatibility, the STT pipeline stops.

**Required Fix (Long Term)**: Port Whisper to pure Go.
- Option A: `github.com/ggerganov/whisper.cpp` via CGO (gives up "no CGO")
- Option B: ONNX Runtime with exported Whisper model (pure Go via `github.com/yalue/onnxruntime_go`)
- Option C: Keep Python but containerize → `whisper_server` as a Docker sidecar image

**Required Fix (Short Term)**: Explicit health check against the whisper server at startup with clear message:
```
2026-03-11T10:00:00Z WARN  whisper-local STT server unreachable at http://localhost:9001
2026-03-11T10:00:00Z WARN  Transcriptions will not be available. Start whisper_server.py
```

---

### 4. No TURN Server — Enterprise Connections Fail

**Problem**: In corporate environments with symmetric NAT, STUN is not sufficient to establish WebRTC connections. Without an integrated or configurable TURN server, aftertalk does not work in:
- Corporate networks with HTTP proxies
- Strict VPN
- Docker networks (symmetric NAT)
- Kubernetes environments

**Not self-contained in these contexts**.

**Required Fix**:
- Option A: Integrate Pion's built-in TURN server (`pion/turn`) → **completely self-contained**
- Option B: Document and configure external TURN server (Coturn) → **declared dependency**

```go
// Option A — embedded TURN server
import "github.com/pion/turn/v3"

func startTURNServer(cfg config.WebRTCConfig) (*turn.Server, error) {
    // Pion has a complete TURN implementation
    // Configurable port, credentials from config.yaml
}
```

---

### 5. Webhook Delivery — Fire-and-Forget Without Persistence

**Problem**: The webhook is delivered in a `go s.deliverWebhook(...)` goroutine without:
- Persistent queue (if the server restarts during delivery, the webhook is lost)
- Retry with backoff (single attempt with 30s timeout only)
- Dead letter queue
- Success/failure log visible via API

The `webhook_events` table exists in the DB but it is unclear whether it is actually used to track deliveries.

**Required Fix**:
```go
// Webhook delivery with persistent retry
type WebhookEvent struct {
    ID          string
    SessionID   string
    Payload     json.RawMessage
    Attempts    int
    LastAttempt time.Time
    Status      string // pending | delivered | failed
    NextRetry   time.Time
}
```

---

### 6. No Verified Offline Mode

**Problem**: The system claims to work with stub providers, but there is no explicitly tested and documented "offline mode". There is no `offline.yaml` configuration profile nor a `--mode=offline` flag.

A user who installs the system without internet does not know which configuration to use to have a working system (even if minimal with stubs).

**Required Fix**:
```bash
# Install flag
./install.sh --mode=offline     # Skip Ollama, skip Whisper download, use stubs
./install.sh --mode=local-ai    # Install Whisper + Ollama (default)
./install.sh --mode=cloud       # Configure cloud STT/LLM providers
```

---

## Real Self-Containment Matrix

| Feature | Self-Contained? | Runtime Dependencies | Fix |
|---|---|---|---|
| HTTP API | ✅ Yes | None | — |
| WebRTC Signaling | ✅ Yes | None | — |
| SQLite DB | ✅ Yes | None | — |
| JWT Auth | ✅ Yes | None | — |
| STT (stub) | ✅ Yes | None | Document |
| STT (whisper-local) | ⚠️ Partial | Python + faster-whisper | Go-native Whisper |
| STT (Google/AWS/Azure) | ❌ Stub | Cloud API + implementation | Implement |
| LLM (stub) | ✅ Yes | None | Document |
| LLM (Ollama) | ⚠️ Partial | Ollama daemon (local) | — |
| LLM (OpenAI/Anthropic) | ✅ Yes | Cloud API (working) | — |
| WebRTC ICE (LAN) | ✅ Yes | None | — |
| WebRTC ICE (WAN) | ⚠️ Partial | Google STUN (hardcoded) | Config STUN |
| WebRTC Symmetric NAT | ❌ No | External TURN server | Pion TURN embedded |
| Webhook Delivery | ⚠️ Partial | External HTTP endpoint | Persistent retry |

---

## Intervention Priority

| # | Gap | Impact | Effort | Priority |
|---|-----|--------|--------|----------|
| 1 | STT cloud = stub, not working | Critical | High | **Critical** |
| 4 | No TURN → symmetric NAT fails | High | Medium | **High** |
| 2 | Hardcoded Google STUN | Medium | Low | **High** |
| 3 | Whisper depends on Python | Medium | High | **Medium** |
| 5 | Fire-and-forget webhook | Medium | Medium | **Medium** |
| 6 | No offline mode | Low | Low | **Low** |

---

## Implementation Steps

### Step 1 — Implement real Google STT (4-6h)

```go
// internal/ai/stt/google.go — real implementation
func (p *GoogleSTTProvider) Transcribe(ctx context.Context, audio *AudioData) (*TranscriptionResult, error) {
    // 1. Get OAuth2 token from credentials.json
    // 2. Encode audio in base64
    // 3. POST to https://speech.googleapis.com/v1/speech:recognize
    // 4. Parse JSON response → TranscriptionSegment[]
}
```

### Step 2 — Embedded TURN Server (6-8h)

```go
// cmd/aftertalk/main.go — start TURN if configured
if cfg.WebRTC.TURN.Enabled {
    turnServer, err := webrtc.StartTURNServer(cfg.WebRTC.TURN)
    // Pion turn: github.com/pion/turn/v3
}
```

### Step 3 — STUN/TURN Config (1h)

Add `WebRTCConfig` and read it in `peer.go` (see installer document).

### Step 4 — Webhook Retry Queue (3-4h)

```go
// Use the existing webhook_events table for a persistent queue
// Worker goroutine that reads pending events and retries with backoff
```
