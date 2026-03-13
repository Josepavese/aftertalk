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
  "timestamp": "2026-03-13T12:00:00Z"
}
```

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

## Security Checklist

- **Always use HTTPS** for the webhook URL in production
- **Validate `X-Aftertalk-Signature`** in `notify_pull` mode before acting on the notification
- **Use `notify_pull`** instead of `push` when handling HIPAA/GDPR-protected data
- **Set `pull_base_url`** to an externally reachable HTTPS address, not `localhost`
- **Secret must be at least 32 bytes** — shorter secrets are accepted but not recommended
