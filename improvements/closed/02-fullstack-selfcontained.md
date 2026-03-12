# Improvement: Fullstack Self-Contained

## Verdetto Avvocato del Diavolo

**L'asserzione "installi e usi, nulla di esterno" è FALSA per la maggior parte dei casi d'uso reali.**

Il core Go è self-contained. Ma la pipeline funzionante di trascrizione + minuta richiede obbligatoriamente servizi esterni. L'asserzione è vera solo in modalità "stub" (demo inutilizzabile in produzione).

---

## Gaps Identificati

### 1. STT Providers Cloud — Stub, Non Implementazioni Reali

**Problema critico**: Google, AWS e Azure STT sono **stub con placeholder**, non implementazioni reali.

```go
// internal/ai/stt/providers.go — Google "provider"
func (p *GoogleSTTProvider) Transcribe(...) (*TranscriptionResult, error) {
    segment := &TranscriptionSegment{
        Text: "[Transcription placeholder - Google STT integration required]",
        // ↑ PLACEHOLDER, non una vera chiamata API
    }
}
```

**Impatto**: Se un utente configura `stt.provider: google` e fornisce credenziali, riceverà placeholder text invece di trascrizioni reali. Non c'è nessun warning visibile che sia un placeholder.

**Solo whisper-local è funzionante** (via HTTP al Python server). Ma whisper-local richiede Python + faster-whisper come dipendenza esterna.

**Fix Richiesto**:
Implementare le chiamate HTTP reali per ogni provider:

```go
// Google Speech-to-Text REST API
POST https://speech.googleapis.com/v1/speech:recognize
Authorization: Bearer {access_token}
Content-Type: application/json
{
  "config": {"encoding": "LINEAR16", "sampleRateHertz": 16000, "languageCode": "it-IT"},
  "audio": {"content": "{base64_audio}"}
}
```

```go
// AWS Transcribe — Streaming API o StartTranscriptionJob
// Azure Speech Services — Batch Transcription API
```

---

### 2. STUN Server Hardcoded — Google (Privacy + Availability)

**Problema**: WebRTC usa esclusivamente `stun:stun.l.google.com:19302` hardcoded.

```go
// internal/bot/webrtc/peer.go:37
{URLs: []string{"stun:stun.l.google.com:19302"}},
```

**Implicazioni**:
1. **Privacy**: Ogni connessione WebRTC rivela l'IP pubblico del server a Google
2. **Affidabilità**: Se il STUN Google è irraggiungibile (firewall aziendali), WebRTC non funziona
3. **Ambienti privati**: Reti air-gapped o corporate non possono usare STUN pubblici
4. **TURN mancante**: In presenza di NAT simmetrico (comune in enterprise) senza TURN server le connessioni falliscono completamente

**Fix Richiesto**:
```yaml
# config.yaml
webrtc:
  ice_servers:
    - urls: ["stun:stun.l.google.com:19302"]  # default, configurabile
  turn_servers: []  # opzionale
  # turn_servers:
  #   - urls: ["turn:turn.example.com:3478"]
  #     username: "user"
  #     credential: "password"
```

```go
// peer.go — leggere da config
iceServers := make([]webrtc.ICEServer, 0, len(cfg.WebRTC.ICEServers))
for _, s := range cfg.WebRTC.ICEServers {
    iceServers = append(iceServers, webrtc.ICEServer{URLs: s.URLs, ...})
}
```

---

### 3. Whisper-Local — Dipendenza Python Non Eliminabile

**Problema**: La pipeline STT di default richiede Python 3.9+, faster-whisper, ffmpeg come dipendenze a runtime esterne al binario Go.

```
aftertalk-server (Go, self-contained)
    ↓ HTTP POST localhost:9001
whisper_server.py (Python, dipendenza esterna)
    ↓ uses
faster-whisper (pip package)
    ↓ uses
ffmpeg (system binary)
```

**Non è "nulla di esterno"**: l'utente deve avere Python funzionante. Se Python o pip si aggiornano e rompono la compatibilità, la pipeline STT si interrompe.

**Fix Richiesto (Lungo Termine)**: Portare Whisper in Go puro.
- Opzione A: `github.com/ggerganov/whisper.cpp` via CGO (rinuncia al "no CGO")
- Opzione B: ONNX Runtime con modello Whisper esportato (Go puro via `github.com/yalue/onnxruntime_go`)
- Opzione C: Mantenere Python ma containerizzare → `whisper_server` come immagine Docker sidecar

**Fix Richiesto (Breve Termine)**: Health check esplicito al whisper server all'avvio con messaggio chiaro:
```
2026-03-11T10:00:00Z WARN  whisper-local STT server non raggiungibile su http://localhost:9001
2026-03-11T10:00:00Z WARN  Le trascrizioni non saranno disponibili. Avviare whisper_server.py
```

---

### 4. Nessun TURN Server — Connessioni Enterprise Falliscono

**Problema**: In ambienti corporate con NAT simmetrico, STUN non è sufficiente per stabilire connessioni WebRTC. Senza TURN server integrato o configurabile, aftertalk non funziona in:
- Reti aziendali con proxy HTTP
- VPN strict
- Docker networks (NAT simmetrico)
- Ambienti Kubernetes

**Non è autoconsistente in questi contesti**.

**Fix Richiesto**:
- Opzione A: Integrare Pion's built-in TURN server (`pion/turn`) → **completamente self-contained**
- Opzione B: Documentare e configurare TURN server esterno (Coturn) → **dipendenza dichiarata**

```go
// Opzione A — TURN server embedded
import "github.com/pion/turn/v3"

func startTURNServer(cfg config.WebRTCConfig) (*turn.Server, error) {
    // Pion ha un'implementazione TURN completa
    // Porta configurabile, credenziali da config.yaml
}
```

---

### 5. Webhook Delivery — Fire-and-Forget senza Persistenza

**Problema**: Il webhook viene consegnato in un goroutine `go s.deliverWebhook(...)` senza:
- Coda persistente (se il server si riavvia durante la delivery, il webhook è perso)
- Retry con backoff (solo tentativo singolo con timeout 30s)
- Dead letter queue
- Registro di successo/fallimento visibile tramite API

La tabella `webhook_events` esiste nel DB ma non è chiaro se venga effettivamente usata per tracciare le consegne.

**Fix Richiesto**:
```go
// Webhook delivery con retry persistente
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

### 6. Nessuna Modalità Offline Verificata

**Problema**: Il sistema afferma di funzionare con stub providers, ma non esiste un "modalità offline" esplicitamente testata e documentata. Non c'è un profilo di configurazione `offline.yaml` né un flag `--mode=offline`.

Un utente che installa il sistema senza internet non sa quale configurazione usare per avere un sistema funzionante (anche se minimale con stub).

**Fix Richiesto**:
```bash
# Install flag
./install.sh --mode=offline     # Skip Ollama, skip Whisper download, use stubs
./install.sh --mode=local-ai    # Install Whisper + Ollama (default)
./install.sh --mode=cloud       # Configure cloud STT/LLM providers
```

---

## Matrice di Self-Containment Reale

| Feature | Self-Contained? | Dipendenze Runtime | Fix |
|---|---|---|---|
| HTTP API | ✅ Sì | Nessuna | — |
| WebRTC Signaling | ✅ Sì | Nessuna | — |
| SQLite DB | ✅ Sì | Nessuna | — |
| JWT Auth | ✅ Sì | Nessuna | — |
| STT (stub) | ✅ Sì | Nessuna | Documentare |
| STT (whisper-local) | ⚠️ Parziale | Python + faster-whisper | Go-native Whisper |
| STT (Google/AWS/Azure) | ❌ Stub | API Cloud + implementazione | Implementare |
| LLM (stub) | ✅ Sì | Nessuna | Documentare |
| LLM (Ollama) | ⚠️ Parziale | Ollama daemon (locale) | — |
| LLM (OpenAI/Anthropic) | ✅ Sì | API Cloud (funzionante) | — |
| WebRTC ICE (LAN) | ✅ Sì | Nessuna | — |
| WebRTC ICE (WAN) | ⚠️ Parziale | STUN Google (hardcoded) | Config STUN |
| WebRTC NAT Simmetrico | ❌ No | TURN server esterno | Pion TURN embedded |
| Webhook Delivery | ⚠️ Parziale | HTTP endpoint esterno | Retry persistente |

---

## Priorità di Intervento

| # | Gap | Impatto | Effort | Priorità |
|---|-----|---------|--------|----------|
| 1 | STT cloud = stub, non funzionante | Critico | Alto | **Critica** |
| 4 | No TURN → NAT simmetrico fallisce | Alto | Medio | **Alta** |
| 2 | STUN hardcoded Google | Medio | Basso | **Alta** |
| 3 | Whisper dipende da Python | Medio | Alto | **Media** |
| 5 | Webhook fire-and-forget | Medio | Medio | **Media** |
| 6 | Nessuna modalità offline | Basso | Basso | **Bassa** |

---

## Passi di Implementazione

### Step 1 — Implementare Google STT reale (4-6h)

```go
// internal/ai/stt/google.go — implementazione reale
func (p *GoogleSTTProvider) Transcribe(ctx context.Context, audio *AudioData) (*TranscriptionResult, error) {
    // 1. Get OAuth2 token da credentials.json
    // 2. Encode audio in base64
    // 3. POST a https://speech.googleapis.com/v1/speech:recognize
    // 4. Parse response JSON → TranscriptionSegment[]
}
```

### Step 2 — TURN Server Embedded (6-8h)

```go
// cmd/aftertalk/main.go — avviare TURN se configurato
if cfg.WebRTC.TURN.Enabled {
    turnServer, err := webrtc.StartTURNServer(cfg.WebRTC.TURN)
    // Pion turn: github.com/pion/turn/v3
}
```

### Step 3 — Config STUN/TURN (1h)

Aggiungere `WebRTCConfig` e leggere in `peer.go` (vedi documento installer).

### Step 4 — Webhook Retry Queue (3-4h)

```go
// Usare la tabella webhook_events esistente per una coda persistente
// Worker goroutine che legge eventi pending e riprova con backoff
```
