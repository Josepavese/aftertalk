# REST API

Base URL: `http://localhost:8080`

## Autenticazione

Tutti gli endpoint `/v1/*` richiedono l'header:
```
X-API-Key: your-api-key
```

Eccezione: `GET /v1/minutes/pull/{token}` — il token nell'URL è la credential.

---

## Health

### GET /v1/health
```bash
curl http://localhost:8080/v1/health
# → {"status":"ok"}
```

### GET /v1/ready
```bash
curl http://localhost:8080/v1/ready
# → {"status":"ready"}
```

---

## Config

### GET /v1/config
Restituisce i template disponibili e l'ID del template di default.
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/config
# → {"templates":[...],"default_template_id":"therapy"}
```

### GET /v1/rtc-config
Restituisce i server ICE da passare a `RTCPeerConnection`.
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/rtc-config
# → {"iceServers":[{"urls":["stun:stun.l.google.com:19302"]}]}
```

---

## Sessions

### POST /v1/sessions
Crea una nuova sessione con i partecipanti.

```bash
curl -X POST http://localhost:8080/v1/sessions \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "participant_count": 2,
    "template_id": "therapy",
    "participants": [
      {"user_id": "dott-rossi", "role": "therapist"},
      {"user_id": "paziente-1", "role": "patient"}
    ]
  }'
```

Risposta:
```json
{
  "session_id": "uuid",
  "participants": [
    {
      "id": "uuid",
      "user_id": "dott-rossi",
      "role": "therapist",
      "token": "eyJ...",
      "expires_at": "2026-03-13T14:00:00Z"
    },
    ...
  ]
}
```

**Validazione:**
- Almeno 2 partecipanti
- `user_id` max 128 char, `role` max 64 char
- `template_id` opzionale; se omesso usa il template di default

### GET /v1/sessions
Lista sessioni con paginazione.

```bash
curl -H "X-API-Key: $KEY" \
  "http://localhost:8080/v1/sessions?status=completed&limit=20&offset=0"
# → {"sessions":[...],"total":42,"limit":20,"offset":0}
```

Parametri query:
- `status`: filtra per stato (`active`, `ended`, `processing`, `completed`, `error`)
- `limit`: max 200, default 50
- `offset`: default 0

### GET /v1/sessions/{id}
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/sessions/uuid
```

### GET /v1/sessions/{id}/status
Risposta compatta `{id, status}`.
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/sessions/uuid/status
# → {"id":"uuid","status":"completed"}
```

### POST /v1/sessions/{id}/end
Termina la sessione. In background: trascrive l'audio rimanente, genera le minute, chiama il webhook.

```bash
curl -X POST -H "X-API-Key: $KEY" http://localhost:8080/v1/sessions/uuid/end
# → 204 No Content
```

**Idempotente**: chiamate multiple su una sessione già terminata restituiscono 204 senza errore.

### DELETE /v1/sessions/{id}
Elimina sessione e dati associati. Fallisce se la sessione è ancora `active`.

```bash
curl -X DELETE -H "X-API-Key: $KEY" http://localhost:8080/v1/sessions/uuid
# → 204 No Content
```

---

## Transcriptions

### GET /v1/transcriptions?session_id={id}
```bash
curl -H "X-API-Key: $KEY" \
  "http://localhost:8080/v1/transcriptions?session_id=uuid&limit=50&offset=0"
```

### GET /v1/transcriptions/{id}
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/transcriptions/uuid
```

---

## Minutes

### GET /v1/minutes?session_id={id}
```bash
curl -H "X-API-Key: $KEY" \
  "http://localhost:8080/v1/minutes?session_id=uuid"
```

Risposta (struttura sections dipende dal template):
```json
{
  "id": "uuid",
  "session_id": "uuid",
  "template_id": "therapy",
  "version": 1,
  "status": "ready",
  "provider": "openai",
  "sections": {
    "themes": ["Ansia da prestazione", "Relazioni familiari"],
    "contents_reported": [
      {"text": "Il paziente riferisce...", "timestamp": 1200}
    ],
    "progress_issues": {
      "progress": ["Miglioramento del sonno"],
      "issues": ["Ancora difficoltà nelle relazioni"]
    },
    "next_steps": ["Esercizio di respirazione quotidiano"]
  },
  "citations": [
    {"timestamp_ms": 1200, "text": "non riesco a dormire", "role": "patient"}
  ],
  "generated_at": "2026-03-13T12:00:00Z"
}
```

### GET /v1/minutes/{id}
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/minutes/uuid
```

### PUT /v1/minutes/{id}
Aggiorna le minute. Crea automaticamente un record di history con la versione precedente.

```bash
curl -X PUT http://localhost:8080/v1/minutes/uuid \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: dott-rossi" \
  -d '{
    "sections": {
      "themes": ["Ansia da prestazione"],
      "next_steps": ["Esercizio respirazione 2x/giorno"]
    },
    "citations": []
  }'
```

`X-User-ID` è opzionale; se omesso viene salvato come `"unknown"`.

### DELETE /v1/minutes/{id}
```bash
curl -X DELETE -H "X-API-Key: $KEY" http://localhost:8080/v1/minutes/uuid
# → 204 No Content
```

### GET /v1/minutes/{id}/versions
Storico delle modifiche.
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/minutes/uuid/versions
# → [{"id":"...","minutes_id":"...","version":1,"content":"{...}","edited_at":"...","edited_by":"..."}]
```

---

## Notify-Pull: GET /v1/minutes/pull/{token}

**No API key richiesta** — il token è la credential.

Usato nel flusso `notify_pull`. Il token è single-use e scade dopo `token_ttl` (default: 1h).

```bash
curl http://localhost:8080/v1/minutes/pull/TOKEN
# → {minutes JSON} oppure 404 se token invalido/scaduto/già usato
```

Tutti gli errori restituiscono `404` indistinguibilmente (prevenzione oracle attack).

Vedere [webhook.md](webhook.md) per il flusso completo.

---

## WebSocket / Signaling

### GET /signaling  (o /ws)
WebSocket per la segnalazione WebRTC. Autenticazione via JWT token nel query string.

```
ws://localhost:8080/signaling?token=eyJ...
```

Messaggi inviati dal client:
```json
{"type": "offer", "sdp": "v=0..."}
{"type": "ice-candidate", "candidate": {...}}
```

Messaggi ricevuti dal server:
```json
{"type": "answer", "sdp": "v=0..."}
{"type": "ice-candidate", "candidate": {...}}
```

---

## Test / Demo

### POST /test/start
Crea o unisce una sessione tramite codice stanza. Richiede API key se configurata.

```bash
curl -X POST http://localhost:8080/test/start \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"code":"stanza-01","name":"Dott. Rossi","role":"therapist","template_id":"therapy"}'
# → {"session_id":"uuid","token":"eyJ..."}
```

Se il ruolo è già occupato da un altro utente: `409 Conflict`.

### GET /demo/config
Restituisce template e (se `demo.enabled=true`) l'API key. **Solo per sviluppo locale.**

### GET /v1/openapi.yaml
Spec OpenAPI completa (servita dal file `specs/contracts/api.yaml`).
