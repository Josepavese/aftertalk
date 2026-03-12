# Improvement 05: WebRTC TURN/STUN — Piano Omnicomprensivo

## Obiettivo

Rendere aftertalk completamente self-contained per WebRTC in qualsiasi rete:
LAN, WAN, NAT simmetrico, corporate firewall, Docker, Kubernetes.
Zero dipendenze esterne per la connettività WebRTC.

---

## Stato Attuale

| Componente | Stato |
|---|---|
| `WebRTCConfig.ICEServers` in config.go | ✅ Già presente |
| ICEServers passati a botServer e peer | ✅ Già presente |
| Default STUN Google hardcoded in config.go | ⚠️ Configurabile ma default fisso |
| TURN server embedded | ❌ Assente |
| ICE servers esposti via API (SSOT) | ❌ Assente (solo `api_key` in `/demo/config`) |
| Credenziali TURN nel JS SDK | ❌ Hardcoded STUN in test-ui |
| Sicurezza TURN (credenziali time-limited) | ❌ Assente |

---

## Architettura Proposta

```
┌─────────────────────────────────────────────────────────────┐
│  config.yaml (SSOT)                                         │
│  webrtc:                                                     │
│    ice_servers: [...]          ← configurabile              │
│    turn:                                                     │
│      enabled: true             ← embedded TURN              │
│      listen_addr: 0.0.0.0:3478                              │
│      public_ip: 1.2.3.4        ← rilevabile auto           │
│      realm: aftertalk                                       │
│      auth_secret: <random>     ← HMAC time-limited          │
│      auth_ttl: 86400           ← 24h validità              │
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
              (quando STUN non basta)
```

---

## Componenti da Implementare

### 1. Config SSOT — `WebRTCConfig` estesa

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
    AuthTTL    int              `koanf:"auth_ttl"`     // secondi (default 86400)
    // Protocolli
    EnableUDP  bool             `koanf:"enable_udp"`   // default true
    EnableTCP  bool             `koanf:"enable_tcp"`   // default true
}
```

**Default sensati** (in `DefaultConfig()`):
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

### 2. TURN Server Embedded — PAL

**File**: `internal/bot/webrtc/turn.go`

```go
// TURNServer wraps pion/turn/v4 with lifecycle management.
// Implements PAL: Business logic consuma solo StartTURNServer().
type TURNServer struct {
    server *turn.Server
    cfg    config.TURNServerConfig
}

func StartTURNServer(ctx context.Context, cfg config.TURNServerConfig) (*TURNServer, error)
func (s *TURNServer) Close() error
func (s *TURNServer) Addr() string
```

**Autenticazione**: HMAC time-based (RFC 5389 long-term credentials):
```
username = "<timestamp>:<user>"    // timestamp = Unix scadenza
password = base64(HMAC-SHA1(secret, username))
```
Questo è il meccanismo usato da Twilio, Xirsys, coturn — è lo standard de facto.

**Auto-detect public IP**: se `PublicIP` è vuoto, chiamata a `api.ipify.org` una sola volta all'avvio.

---

### 3. REST API — Nuovo Endpoint `/v1/rtc-config`

**Endpoint**: `GET /v1/rtc-config`
**Auth**: Bearer token (API key) — le credenziali TURN sono sensibili
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

Le credenziali TURN sono **generate on-the-fly** a ogni richiesta (time-limited).
Il campo `ttl` indica al client quando richiederle di nuovo.

**`/demo/config`** (pubblico): espone solo STUN (nessuna credenziale TURN):
```json
{
  "api_key": "...",
  "templates": [...],
  "ice_servers": [{"urls": ["stun:stun.l.google.com:19302"]}]
}
```

---

### 4. SSOT nel JS SDK / test-ui

Il test-ui (e futuro SDK) legge i server ICE dall'API invece di averli hardcoded:

```javascript
// Prima (hardcoded in test-ui):
const pc = new RTCPeerConnection({
    iceServers: [{urls: ['stun:stun.l.google.com:19302']}]  // ← hardcoded
});

// Dopo (SSOT via API):
const rtcConfig = await client.getRTCConfig();  // GET /v1/rtc-config
const pc = new RTCPeerConnection({
    iceServers: rtcConfig.ice_servers  // ← dal server, con TURN se abilitato
});
```

---

### 5. Sicurezza

| Aspetto | Soluzione |
|---|---|
| Credenziali TURN non esposte pubblicamente | `/v1/rtc-config` richiede API key |
| Credenziali time-limited | HMAC username con timestamp di scadenza |
| TURN non relay tutto il traffico | pion/turn autentica ogni sessione |
| Brute force TURN | Rate limiting su `/v1/rtc-config` |
| public_ip auto-detect | Singola chiamata all'avvio, non a ogni request |

---

### 6. Installer (SSOT)

`install.sh` aggiunge la sezione `webrtc:` alla config generata:
```yaml
webrtc:
  ice_servers:
    - urls: ["stun:stun.l.google.com:19302"]
  turn:
    enabled: false          # Abilitare se serve NAT traversal
    listen_addr: "0.0.0.0:3478"
    public_ip: ""           # Lasciare vuoto per auto-detect
    realm: "aftertalk"
    auth_secret: "<RANDOM_32>"   # Generato dall'installer
    auth_ttl: 86400
    enable_udp: true
    enable_tcp: true
```

L'installer genera `auth_secret` con lo stesso meccanismo dell'API key (32 chars random).

---

### 7. Documentazione Operativa

**Quando abilitare TURN**:
- `aftertalk status` mostra se TURN è attivo
- Log all'avvio: `TURN server listening on UDP/TCP :3478 (public: 1.2.3.4)`
- Firewall: aprire porta 3478 UDP+TCP

**Porta TURN configurabile** — non hardcoded 3478.

---

## File Coinvolti

| File | Modifica |
|---|---|
| `internal/config/config.go` | Aggiungere `TURNServerConfig` |
| `internal/bot/webrtc/turn.go` | **Nuovo** — TURN server embedded |
| `internal/bot/webrtc/peer.go` | Nessuna modifica (già legge ICEServers) |
| `internal/api/server.go` | Aggiungere route `GET /v1/rtc-config` |
| `internal/api/handler/rtc.go` | **Nuovo** — handler per rtc-config |
| `cmd/aftertalk/main.go` | Avviare TURN se enabled, iniettare ICE servers aggiornati |
| `cmd/test-ui/index.html` | Leggere ICE servers da `/v1/rtc-config` |
| `scripts/install.sh` | Aggiungere sezione webrtc in config generata |

---

## Ordine di Implementazione

1. **Config** — aggiungere `TURNServerConfig` (5 min)
2. **TURN server** — `internal/bot/webrtc/turn.go` (30 min)
3. **main.go** — avvio TURN + aggiornamento ICEServers con entry TURN (15 min)
4. **Handler** — `GET /v1/rtc-config` con generazione credenziali HMAC (15 min)
5. **test-ui** — leggere ICE servers dall'API (10 min)
6. **installer** — sezione webrtc in config.yaml (10 min)
7. **Test** — unit test HMAC generation + integrazione (20 min)
