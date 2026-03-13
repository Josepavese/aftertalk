# Improvement 05: WebRTC TURN/STUN — Comprehensive Plan

## Objective

Make aftertalk completely self-contained for WebRTC on any network:
LAN, WAN, symmetric NAT, corporate firewall, Docker, Kubernetes.
Zero external dependencies for WebRTC connectivity.

---

## Current State

| Component | Status |
|---|---|
| `WebRTCConfig.ICEServers` in config.go | ✅ Already present |
| ICEServers passed to botServer and peer | ✅ Already present |
| Default Google STUN hardcoded in config.go | ⚠️ Configurable but fixed default |
| Embedded TURN server | ❌ Absent |
| ICE servers exposed via API (SSOT) | ❌ Absent (only `api_key` in `/demo/config`) |
| TURN credentials in JS SDK | ❌ Hardcoded STUN in test-ui |
| TURN security (time-limited credentials) | ❌ Absent |

---

## Proposed Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  config.yaml (SSOT)                                         │
│  webrtc:                                                     │
│    ice_servers: [...]          ← configurable               │
│    turn:                                                     │
│      enabled: true             ← embedded TURN              │
│      listen_addr: 0.0.0.0:3478                              │
│      public_ip: 1.2.3.4        ← auto-detectable           │
│      realm: aftertalk                                       │
│      auth_secret: <random>     ← HMAC time-limited          │
│      auth_ttl: 86400           ← 24h validity               │
└────────────────────────┬────────────────────────────────────┘
                         │
           ┌─────────────┼──────────────┐
           ▼             ▼              ▼
   ┌───────────────┐  ┌──────────┐  ┌────────────────┐
   │ TURN Server   │  │ REST API │  │  JS SDK        │
   │ pion/turn/v4  │  │ /v1/     │  │ @aftertalk/sdk │
   │ UDP+TCP :3478 │  │          │  │                │
   └───────┬───────┘  └────┬─────┘  └───────┬────────┘
           │               │                │
           │    GET /v1/rtc-config           │
           │    → {ice_servers, turn_creds}  │
           │               │────────────────►│
           │                                 │
           ◄─────── TURN relay ──────────────►
              (when STUN is not enough)
```

---

## Components to Implement

### 1. SSOT Config — Extended `WebRTCConfig`

**File**: `internal/config/config.go`

```go
type WebRTCConfig struct {
    ICEServers []ICEServerConfig `koanf:"ice_servers"`
    TURN       TURNServerConfig  `koanf:"turn"`
}

type ICEServerConfig struct {
    URLs       []string `koanf:"urls"`
    Username   string   `koanf:"username,omitempty"`
    Credential string   `koanf:"credential,omitempty"`
}

type TURNServerConfig struct {
    Enabled    bool             `koanf:"enabled"`
    ListenAddr string           `koanf:"listen_addr"`  // "0.0.0.0:3478"
    PublicIP   string           `koanf:"public_ip"`    // "" = auto-detect
    Realm      string           `koanf:"realm"`        // "aftertalk"
    AuthSecret string           `koanf:"auth_secret"`  // HMAC shared secret
    AuthTTL    int              `koanf:"auth_ttl"`     // seconds (default 86400)
    // Protocols
    EnableUDP  bool             `koanf:"enable_udp"`   // default true
    EnableTCP  bool             `koanf:"enable_tcp"`   // default true
}
```

**Sensible defaults** (in `DefaultConfig()`):
```go
WebRTC: WebRTCConfig{
    ICEServers: []ICEServerConfig{
        {URLs: []string{"stun:stun.l.google.com:19302"}},
    },
    TURN: TURNServerConfig{
        Enabled:    false,
        ListenAddr: "0.0.0.0:3478",
        Realm:      "aftertalk",
        AuthTTL:    86400,
        EnableUDP:  true,
        EnableTCP:  true,
    },
},
```

---

### 2. Embedded TURN Server — PAL

**File**: `internal/bot/webrtc/turn.go`

```go
// TURNServer wraps pion/turn/v4 with lifecycle management.
// Implements PAL: business logic only consumes StartTURNServer().
type TURNServer struct {
    server *turn.Server
    cfg    config.TURNServerConfig
}

func StartTURNServer(ctx context.Context, cfg config.TURNServerConfig) (*TURNServer, error)
func (s *TURNServer) Close() error
func (s *TURNServer) Addr() string
```

**Authentication**: HMAC time-based (RFC 5389 long-term credentials):
```
username = "<timestamp>:<user>"    // timestamp = Unix expiry
password = base64(HMAC-SHA1(secret, username))
```
This is the mechanism used by Twilio, Xirsys, coturn — it is the de facto standard.

**Auto-detect public IP**: if `PublicIP` is empty, a single call to `api.ipify.org` at startup.

---

### 3. REST API — New Endpoint `/v1/rtc-config`

**Endpoint**: `GET /v1/rtc-config`
**Auth**: Bearer token (API key) — TURN credentials are sensitive
**Response**:
```json
{
  "ice_servers": [
    {"urls": ["stun:stun.l.google.com:19302"]},
    {
      "urls": ["turn:1.2.3.4:3478", "turn:1.2.3.4:3478?transport=tcp"],
      "username": "1710000000:aftertalk",
      "credential": "base64-hmac-sha1..."
    }
  ],
  "ttl": 86400
}
```

TURN credentials are **generated on-the-fly** on each request (time-limited).
The `ttl` field tells the client when to refresh them.

**`/demo/config`** (public): only exposes STUN (no TURN credentials):
```json
{
  "api_key": "...",
  "templates": [...],
  "ice_servers": [{"urls": ["stun:stun.l.google.com:19302"]}]
}
```

---

### 4. SSOT in JS SDK / test-ui

The test-ui (and future SDK) reads ICE servers from the API instead of hardcoding them:

```javascript
// Before (hardcoded in test-ui):
const pc = new RTCPeerConnection({
    iceServers: [{urls: ['stun:stun.l.google.com:19302']}]  // ← hardcoded
});

// After (SSOT via API):
const rtcConfig = await client.getRTCConfig();  // GET /v1/rtc-config
const pc = new RTCPeerConnection({
    iceServers: rtcConfig.ice_servers  // ← from server, with TURN if enabled
});
```

---

### 5. Security

| Aspect | Solution |
|---|---|
| TURN credentials not publicly exposed | `/v1/rtc-config` requires API key |
| Time-limited credentials | HMAC username with expiry timestamp |
| TURN does not relay all traffic | pion/turn authenticates every session |
| TURN brute force | Rate limiting on `/v1/rtc-config` |
| public_ip auto-detect | Single call at startup, not per request |

---

### 6. Installer (SSOT)

`install.sh` adds the `webrtc:` section to the generated config:
```yaml
webrtc:
  ice_servers:
    - urls: ["stun:stun.l.google.com:19302"]
  turn:
    enabled: false          # Enable if NAT traversal is needed
    listen_addr: "0.0.0.0:3478"
    public_ip: ""           # Leave empty for auto-detect
    realm: "aftertalk"
    auth_secret: "<RANDOM_32>"   # Generated by installer
    auth_ttl: 86400
    enable_udp: true
    enable_tcp: true
```

The installer generates `auth_secret` using the same mechanism as the API key (32 random chars).

---

### 7. Operational Documentation

**When to enable TURN**:
- `aftertalk status` shows whether TURN is active
- Startup log: `TURN server listening on UDP/TCP :3478 (public: 1.2.3.4)`
- Firewall: open port 3478 UDP+TCP

**Configurable TURN port** — not hardcoded to 3478.

---

## Files Involved

| File | Change |
|---|---|
| `internal/config/config.go` | Add `TURNServerConfig` |
| `internal/bot/webrtc/turn.go` | **New** — embedded TURN server |
| `internal/bot/webrtc/peer.go` | No change (already reads ICEServers) |
| `internal/api/server.go` | Add route `GET /v1/rtc-config` |
| `internal/api/handler/rtc.go` | **New** — handler for rtc-config |
| `cmd/aftertalk/main.go` | Start TURN if enabled, inject updated ICE servers |
| `cmd/test-ui/index.html` | Read ICE servers from `/v1/rtc-config` |
| `scripts/install.sh` | Add webrtc section in generated config |

---

## Implementation Order

1. **Config** — add `TURNServerConfig` (5 min)
2. **TURN server** — `internal/bot/webrtc/turn.go` (30 min)
3. **main.go** — TURN startup + ICEServers update with TURN entry (15 min)
4. **Handler** — `GET /v1/rtc-config` with HMAC credential generation (15 min)
5. **test-ui** — read ICE servers from API (10 min)
6. **installer** — webrtc section in config.yaml (10 min)
7. **Tests** — unit test HMAC generation + integration (20 min)
