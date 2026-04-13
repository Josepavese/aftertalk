# Webhook Integration

Aftertalk delivers minutes to your backend system via HTTP webhook after each session ends.

## Modes

Configure via `AFTERTALK_WEBHOOK_MODE` (or `webhook.mode` in YAML):

| Mode | Description |
|---|---|
| `push` | Full minutes JSON sent directly in POST body (default, legacy) |
| `notify_pull` | Only a signed retrieval URL is sent; you pull the data separately |

`notify_pull` is recommended for sensitive contexts (HIPAA/GDPR): no clinical data leaves the server until you actively pull it.

---

## Push Mode

```yaml
webhook:
  url: "https://your-system.example.com/aftertalk/webhook"
  mode: push
  timeout: 30s
```

Request sent by Aftertalk to your endpoint:

```http
POST https://your-system.example.com/aftertalk/webhook
Content-Type: application/json

{
  "session_id": "uuid",
  "session_metadata": "{\"appointment_id\":\"appt_123\",\"doctor_id\":\"doc_456\",\"patient_id\":\"pat_789\"}",
  "participants": [
    { "user_id": "doc_456", "role": "therapist" },
    { "user_id": "pat_789", "role": "patient" }
  ],
  "minutes": {
    "id": "uuid",
    "session_id": "uuid",
    "template_id": "therapy",
    "version": 1,
    "status": "ready",
    "sections": { ... },
    "citations": [ ... ],
    "generated_at": "2026-03-13T12:00:00Z"
  },
  "timestamp": "2026-03-13T12:00:00Z"
}
```

**Fields:**

| Field | Type | Description |
|---|---|---|
| `session_id` | string | Aftertalk session UUID |
| `session_metadata` | string (JSON) | Opaque string passed at session creation. Aftertalk never inspects it. Use it to carry your own context (appointment ID, doctor ID, etc.). Omitted if empty. |
| `participants` | array | Compact participant list `[{user_id, role}]` set at session creation. Omitted if empty. |
| `minutes` | object | Structured minutes output (sections, citations) |
| `timestamp` | string (RFC3339) | Delivery timestamp |

Your endpoint must return `2xx`. Non-2xx or network errors trigger the retrier (see below).

---

## Notify-Pull Mode

```yaml
webhook:
  url: "https://your-system.example.com/aftertalk/webhook"
  mode: notify_pull
  secret: "your-hmac-secret-min-32-bytes"     # required for signature verification
  pull_base_url: "https://aftertalk.yourdomain.com"  # base URL for the pull link
  token_ttl: 1h                                       # token validity (default: 1h)
```

### Step 1 — Notification (Aftertalk → your server)

```http
POST https://your-system.example.com/aftertalk/webhook
Content-Type: application/json
X-Aftertalk-Signature: hmac-sha256=<hex>

{
  "session_id": "uuid",
  "retrieve_url": "https://aftertalk.yourdomain.com/v1/minutes/pull/TOKEN",
  "expires_at": "2026-03-13T13:00:00Z",
  "timestamp": "2026-03-13T12:00:00Z",
  "session_metadata": "{\"appointment_id\":\"appt_123\",\"doctor_id\":\"doc_456\"}",
  "participants": [
    { "user_id": "doc_456", "role": "therapist" },
    { "user_id": "pat_789", "role": "patient" }
  ]
}
```

The notification payload carries the full session context (`session_metadata`, `participants`) so your server can route the event (e.g. notify the right doctor) without first pulling the minutes. The `retrieve_url` points to the actual clinical data — it is fetched only when needed.

### Step 2 — Verify signature (your server)

```python
import hmac, hashlib

def verify(body: bytes, signature: str, secret: str) -> bool:
    mac = hmac.new(secret.encode(), body, hashlib.sha256)
    expected = "hmac-sha256=" + mac.hexdigest()
    return hmac.compare_digest(expected, signature)
```

```go
mac := hmac.New(sha256.New, []byte(webhookSecret))
mac.Write(requestBody)
expected := "hmac-sha256=" + hex.EncodeToString(mac.Sum(nil))
if !hmac.Equal([]byte(expected), []byte(r.Header.Get("X-Aftertalk-Signature"))) {
    http.Error(w, "invalid signature", 401)
}
```

### Step 3 — Pull data (your server → Aftertalk)

```bash
curl https://aftertalk.yourdomain.com/v1/minutes/pull/TOKEN
# → full minutes JSON (same shape as push mode "minutes" field)
# → 404 if token expired, already used, or invalid
```

**Important**: the token is single-use. After the first successful pull, it's consumed and the data is purged. All errors return `404` (oracle attack prevention — no distinction between expired, used, or invalid).

---

## Retry Logic

The webhook retrier uses exponential backoff with jitter. Configuration:

```yaml
webhook:
  max_retries: 3         # max delivery attempts (default: 3)
  retry_delay: 5s        # initial delay between attempts (default: 5s)
  retry_multiplier: 2.0  # backoff multiplier (default: 2.0)
```

Failed attempts are stored in the `webhook_events` table with status `failed`. After exhausting retries, the event remains as `failed` — no alert is emitted by default.

---

## Session Context: Metadata and Participants

Both `push` and `notify_pull` payloads carry a **session context** — the `session_metadata` string and the `participants` array — set once at session-creation time and propagated unchanged to every delivery.

### Why

Without this, recipients must maintain a local `session_id → context` mapping table to associate an incoming webhook with their own data model (appointment, patient, etc.). The session context eliminates that need.

### Setting the context at creation

```bash
curl -X POST http://localhost:8080/v1/sessions \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "participant_count": 2,
    "template_id": "therapy",
    "metadata": "{\"appointment_id\":\"appt_123\",\"doctor_id\":\"doc_456\",\"patient_id\":\"pat_789\"}",
    "participants": [
      {"user_id": "doc_456", "role": "therapist"},
      {"user_id": "pat_789", "role": "patient"}
    ]
  }'
```

The `metadata` field is an **opaque JSON string**. Aftertalk stores it as-is and echoes it back in every webhook payload under `session_metadata`. Use any structure that makes sense for your application.

### Receiving the context in your webhook handler

```php
// PHP example — Laravel/Slim controller
public function handleWebhook(Request $request): Response
{
    // 1. Verify signature (notify_pull mode)
    $signature = $request->header('X-Aftertalk-Signature');
    if (!$this->aftertalk->webhook->verifySignature($request->getContent(), $signature)) {
        return response('Forbidden', 403);
    }

    // 2. Parse payload
    $payload = json_decode($request->getContent(), true);
    $meta    = json_decode($payload['session_metadata'] ?? '{}', true);

    // 3. Associate with your data model — no lookup table needed
    $appointmentId = $meta['appointment_id'];
    $doctorId      = $meta['doctor_id'];

    // For notify_pull: retrieve_url is in payload; pull minutes separately.
    // For push: minutes are already in payload['minutes'].

    Appointment::find($appointmentId)->attachMinutes($payload);
    Doctor::find($doctorId)->notifyMinutesReady($appointmentId);

    return response('OK', 200);
}
```

### Security notes

- `session_metadata` is **server-supplied**: it is set only via the `POST /v1/sessions` endpoint, which requires the API key. Clients (browsers) never touch it.
- The content is **not validated or parsed** by Aftertalk. Ensure your backend sanitises it before using it in queries or rendering.
- In `notify_pull` mode the metadata travels in the notification (before the pull). If the metadata itself is sensitive, consider using only opaque IDs (e.g. `appointment_id`) and resolving them server-side.

---

## Security Checklist

- **Always use HTTPS** for the webhook URL in production
- **Validate `X-Aftertalk-Signature`** in `notify_pull` mode before acting on the notification
- **Use `notify_pull`** instead of `push` when handling HIPAA/GDPR-protected data
- **Set `pull_base_url`** to an externally reachable HTTPS address, not `localhost`
- **Secret must be at least 32 bytes** — shorter secrets are accepted but not recommended
