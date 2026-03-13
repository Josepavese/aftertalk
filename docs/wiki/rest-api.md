# REST API

Base URL: `http://localhost:8080`

## Authentication

All `/v1/*` endpoints require the header:
```
X-API-Key: your-api-key
```

Exception: `GET /v1/minutes/pull/{token}` — the token in the URL is the credential.

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
Returns available templates and the default template ID.
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/config
# → {"templates":[...],"default_template_id":"therapy"}
```

### GET /v1/rtc-config
Returns ICE servers to pass to `RTCPeerConnection`.
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/rtc-config
# → {"iceServers":[{"urls":["stun:stun.l.google.com:19302"]}]}
```

---

## Sessions

### POST /v1/sessions
Creates a new session with participants.

```bash
curl -X POST http://localhost:8080/v1/sessions \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "participant_count": 2,
    "template_id": "therapy",
    "participants": [
      {"user_id": "dr-smith", "role": "therapist"},
      {"user_id": "patient-1", "role": "patient"}
    ]
  }'
```

Response:
```json
{
  "session_id": "uuid",
  "participants": [
    {
      "id": "uuid",
      "user_id": "dr-smith",
      "role": "therapist",
      "token": "eyJ...",
      "expires_at": "2026-03-13T14:00:00Z"
    },
    ...
  ]
}
```

**Validation:**
- At least 2 participants
- `user_id` max 128 chars, `role` max 64 chars
- `template_id` optional; defaults to the configured default template

### GET /v1/sessions
List sessions with pagination.

```bash
curl -H "X-API-Key: $KEY" \
  "http://localhost:8080/v1/sessions?status=completed&limit=20&offset=0"
# → {"sessions":[...],"total":42,"limit":20,"offset":0}
```

Query parameters:
- `status`: filter by status (`active`, `ended`, `processing`, `completed`, `error`)
- `limit`: max 200, default 50
- `offset`: default 0

### GET /v1/sessions/{id}
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/sessions/uuid
```

### GET /v1/sessions/{id}/status
Compact response `{id, status}`.
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/sessions/uuid/status
# → {"id":"uuid","status":"completed"}
```

### POST /v1/sessions/{id}/end
Ends the session. In the background: transcribes remaining audio, generates minutes, calls the webhook.

```bash
curl -X POST -H "X-API-Key: $KEY" http://localhost:8080/v1/sessions/uuid/end
# → 204 No Content
```

**Idempotent**: multiple calls on an already-ended session return 204 without error.

### DELETE /v1/sessions/{id}
Deletes the session and associated data. Fails if the session is still `active`.

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

Response (sections structure depends on the template):
```json
{
  "id": "uuid",
  "session_id": "uuid",
  "template_id": "therapy",
  "version": 1,
  "status": "ready",
  "provider": "openai",
  "sections": {
    "themes": ["Performance anxiety", "Family relationships"],
    "contents_reported": [
      {"text": "The patient reports...", "timestamp": 1200}
    ],
    "progress_issues": {
      "progress": ["Improved sleep"],
      "issues": ["Still struggling with relationships"]
    },
    "next_steps": ["Daily breathing exercise"]
  },
  "citations": [
    {"timestamp_ms": 1200, "text": "I can't sleep", "role": "patient"}
  ],
  "generated_at": "2026-03-13T12:00:00Z"
}
```

### GET /v1/minutes/{id}
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/minutes/uuid
```

### PUT /v1/minutes/{id}
Updates the minutes. Automatically creates a history record with the previous version.

```bash
curl -X PUT http://localhost:8080/v1/minutes/uuid \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: dr-smith" \
  -d '{
    "sections": {
      "themes": ["Performance anxiety"],
      "next_steps": ["Breathing exercise 2x/day"]
    },
    "citations": []
  }'
```

`X-User-ID` is optional; if omitted it is stored as `"unknown"`.

### DELETE /v1/minutes/{id}
```bash
curl -X DELETE -H "X-API-Key: $KEY" http://localhost:8080/v1/minutes/uuid
# → 204 No Content
```

### GET /v1/minutes/{id}/versions
History of edits.
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/minutes/uuid/versions
# → [{"id":"...","minutes_id":"...","version":1,"content":"{...}","edited_at":"...","edited_by":"..."}]
```

---

## Notify-Pull: GET /v1/minutes/pull/{token}

**No API key required** — the token is the credential.

Used in the `notify_pull` flow. The token is single-use and expires after `token_ttl` (default: 1h).

```bash
curl http://localhost:8080/v1/minutes/pull/TOKEN
# → {minutes JSON} or 404 if token is invalid/expired/already used
```

All errors return `404` indistinguishably (prevents oracle attacks).

See [webhook.md](webhook.md) for the complete flow.

---

## WebSocket / Signaling

### GET /signaling  (or /ws)
WebSocket for WebRTC signaling. Authenticated via JWT token in the query string.

```
ws://localhost:8080/signaling?token=eyJ...
```

Messages sent by the client:
```json
{"type": "offer", "sdp": "v=0..."}
{"type": "ice-candidate", "candidate": {...}}
```

Messages received from the server:
```json
{"type": "answer", "sdp": "v=0..."}
{"type": "ice-candidate", "candidate": {...}}
```

---

## Test / Demo

### POST /test/start
Creates or joins a session via room code. Requires API key if configured.

```bash
curl -X POST http://localhost:8080/test/start \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"code":"room-01","name":"Dr. Smith","role":"therapist","template_id":"therapy"}'
# → {"session_id":"uuid","token":"eyJ..."}
```

If the role is already taken by another user: `409 Conflict`.

### GET /demo/config
Returns templates and (if `demo.enabled=true`) the API key. **For local development only.**

### GET /v1/openapi.yaml
Full OpenAPI spec (served from `specs/contracts/api.yaml`).
