# REST API

Base URL: `http://localhost:8080`

## Authentication

All `/v1/*` endpoints require the header:
```
Authorization: Bearer your-api-key
```

Exceptions:
- `GET /v1/config`
- `GET /v1/rtc-config`
- `GET /v1/minutes/pull/{token}`

---

## Health

### GET /v1/health
```bash
curl -H "Authorization: Bearer $KEY" http://localhost:8080/v1/health
# → {"status":"ok","version":"1.0.0","commit":"...","tag":"edge","build_time":"...","build_source":"github-actions"}
```

### GET /v1/version
```bash
curl -H "Authorization: Bearer $KEY" http://localhost:8080/v1/version
# → {"version":"1.0.0","commit":"...","tag":"edge","build_time":"...","build_source":"github-actions"}
```

### GET /v1/ready
```bash
curl -H "Authorization: Bearer $KEY" http://localhost:8080/v1/ready
# → {"status":"ready"}
```

---

## Config

### GET /v1/config
Returns available templates and the default template ID.
```bash
curl http://localhost:8080/v1/config
# → {"templates":[...],"default_template_id":"therapy"}
```

### GET /v1/rtc-config
Returns ICE servers to pass to `RTCPeerConnection`.
```bash
curl http://localhost:8080/v1/rtc-config
# → {"iceServers":[{"urls":["stun:stun.l.google.com:19302"]}]}
```

---

## Sessions

### POST /v1/sessions
Creates a new session with participants.

```bash
curl -X POST http://localhost:8080/v1/sessions \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "participant_count": 2,
    "template_id": "therapy",
    "metadata": "{\"appointment_id\":\"appt_123\",\"doctor_id\":\"doc_456\",\"patient_id\":\"pat_789\"}",
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

**Request fields:**

| Field | Type | Required | Description |
|---|---|---|---|
| `participant_count` | int | yes | Must match the length of `participants` |
| `participants` | array | yes | `[{user_id, role}]`; min 2 entries |
| `template_id` | string | no | Template for minute structure; defaults to configured default |
| `metadata` | string | no | Opaque JSON string stored with the session and propagated to every webhook delivery. Use it to embed your own context (appointment ID, user IDs, etc.) so webhook recipients can associate the minutes without a lookup table. |

**Validation:**
- At least 2 participants
- `user_id` max 128 chars, `role` max 64 chars
- `template_id` optional; defaults to the configured default template
- `metadata` is stored as-is — Aftertalk never parses or validates its content

### GET /v1/sessions
List sessions with pagination.

```bash
curl -H "Authorization: Bearer $KEY" \
  "http://localhost:8080/v1/sessions?status=completed&limit=20&offset=0"
# → {"sessions":[...],"total":42,"limit":20,"offset":0}
```

Query parameters:
- `status`: filter by status (`active`, `ended`, `processing`, `completed`, `error`)
- `limit`: max 200, default 50
- `offset`: default 0

### GET /v1/sessions/{id}
```bash
curl -H "Authorization: Bearer $KEY" http://localhost:8080/v1/sessions/uuid
```

### GET /v1/sessions/{id}/status
Compact response `{id, status}`.
```bash
curl -H "Authorization: Bearer $KEY" http://localhost:8080/v1/sessions/uuid/status
# → {"id":"uuid","status":"completed"}
```

### POST /v1/sessions/{id}/end
Ends the session. In the background: transcribes remaining audio, generates minutes, calls the webhook.

```bash
curl -X POST -H "Authorization: Bearer $KEY" http://localhost:8080/v1/sessions/uuid/end
# → 204 No Content
```

**Idempotent**: multiple calls on an already-ended session return 204 without error.

### DELETE /v1/sessions/{id}
Deletes the session and associated data. Fails if the session is still `active`.

```bash
curl -X DELETE -H "Authorization: Bearer $KEY" http://localhost:8080/v1/sessions/uuid
# → 204 No Content
```

---

## Transcriptions

### GET /v1/transcriptions?session_id={id}
```bash
curl -H "Authorization: Bearer $KEY" \
  "http://localhost:8080/v1/transcriptions?session_id=uuid&limit=50&offset=0"
```

### GET /v1/transcriptions/{id}
```bash
curl -H "Authorization: Bearer $KEY" http://localhost:8080/v1/transcriptions/uuid
```

---

## Minutes

### GET /v1/minutes?session_id={id}
```bash
curl -H "Authorization: Bearer $KEY" \
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
  "summary": {
    "overview": "The conversation opens with a status check and moves into a short update on recent improvement.",
    "phases": [
      {
        "title": "Opening",
        "summary": "Greeting and start-of-session alignment",
        "start_ms": 0,
        "end_ms": 60000
      },
      {
        "title": "Update",
        "summary": "The patient reports feeling better than the previous day",
        "start_ms": 60000,
        "end_ms": 180000
      }
    ]
  },
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
curl -H "Authorization: Bearer $KEY" http://localhost:8080/v1/minutes/uuid
```

### PUT /v1/minutes/{id}
Updates the minutes. Automatically creates a history record with the previous version.

```bash
curl -X PUT http://localhost:8080/v1/minutes/uuid \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -H "X-User-ID: dr-smith" \
  -d '{
    "summary": {
      "overview": "Updated clinical summary",
      "phases": []
    },
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
curl -X DELETE -H "Authorization: Bearer $KEY" http://localhost:8080/v1/minutes/uuid
# → 204 No Content
```

### GET /v1/minutes/{id}/versions
History of edits.
```bash
curl -H "Authorization: Bearer $KEY" http://localhost:8080/v1/minutes/uuid/versions
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

## TLS Configuration

By default Aftertalk listens on plain HTTP/WS, which is the expected setup when running behind a reverse proxy (Apache, nginx). For standalone deploys without a proxy, native TLS can be enabled via the `tls:` section in `aftertalk.yaml`:

```yaml
tls:
  cert_file: /etc/aftertalk/certs/cert.pem
  key_file:  /etc/aftertalk/certs/key.pem
```

Behavior:
- Both fields set **and files exist on disk** → server starts HTTPS/WSS (`ListenAndServeTLS`).
- Fields set but **files missing** → server exits with an explicit error. It never silently falls back to HTTP.
- Fields empty (default) → plain HTTP/WS.

When TLS is active, the signaling WebSocket endpoint is available as `wss://` instead of `ws://`.

See [deployment.md](deployment.md) for TLS options in production.

---

## WebSocket / Signaling

### GET /signaling  (or /ws)
WebSocket for WebRTC signaling. Authenticated via JWT token in the query string.

```
# plain HTTP
ws://localhost:8080/signaling?token=eyJ...

# with native TLS enabled
wss://yourdomain.com/signaling?token=eyJ...

# behind Apache/nginx reverse proxy
wss://yourdomain.com/aftertalk/signaling?token=eyJ...
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

## Test UI

Aftertalk serves the embedded test UI at the server root:

```bash
open http://localhost:8080/
```

To create or join a room programmatically, use `POST /v1/rooms/join`.

### GET /v1/openapi.yaml
Full OpenAPI spec (served from `specs/contracts/api.yaml`).
