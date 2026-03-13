# 07 — Secure Minutes Delivery: Notify + Pull pattern

## Problem

The current webhook (`pkg/webhook/client.go`) does a **direct push** of the entire structured minutes in the POST body. In a healthcare context this is unacceptable:

| Problem | Impact |
|---|---|
| Sensitive data in webhook body | Exposed to every HTTP intermediary (proxy, CDN, logs) |
| No payload signature | Recipient cannot verify authenticity |
| Minutes not deleted after delivery | Aftertalk becomes an unintentional medical data store |
| Static webhook URL server-side | Not configurable per session / tenant |
| No confirmation that recipient saved data | Silent delivery loss possible |

---

## Solution: Notify + Pull

Inspired by Stripe, HIPAA systems, DocuSign:

```
Aftertalk                    Recipient server
    │                               │
    │─── POST /webhook ────────────>│  {session_id, retrieve_url, expires_at, sig}
    │    (zero sensitive data)      │
    │                               │
    │<── GET /v1/minutes/pull/{tok}─│  HMAC-signed single-use token
    │                               │
    │─── 200 {minutes JSON} ───────>│  transmission over recipient's TLS connection
    │                               │
    │    [delete from DB] ──────────│  Aftertalk becomes a pipeline, not an archive
```

### Security guarantees

- **Confidentiality**: the webhook never carries sensitive data, only a time-limited token
- **Authenticity**: the webhook has header `X-Aftertalk-Signature: hmac-sha256=<sig>` over the configured `webhook_secret` — the recipient verifies before pulling
- **Non-repudiability**: the pull happens over the recipient server's TLS connection, with a single-use token
- **Data minimization**: minutes are deleted from the Aftertalk DB after confirmed pull
- **Replay protection**: single-use token, marked `used` on first successful pull

---

## Implementation

### 1. `retrieval_tokens` table (inline migration in `main.go`)

```sql
CREATE TABLE IF NOT EXISTS retrieval_tokens (
    id          TEXT PRIMARY KEY,
    minutes_id  TEXT NOT NULL,
    expires_at  DATETIME NOT NULL,
    used_at     DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(minutes_id) REFERENCES minutes(id) ON DELETE CASCADE
);
```

### 2. Config (`internal/config/config.go`)

```go
type WebhookConfig struct {
    URL           string        `koanf:"url"`
    Timeout       time.Duration `koanf:"timeout"`
    Secret        string        `koanf:"secret"`         // for HMAC notification signing
    TokenTTL      time.Duration `koanf:"token_ttl"`      // default: 1h
    DeleteOnPull  bool          `koanf:"delete_on_pull"` // default: true
    PullBaseURL   string        `koanf:"pull_base_url"`  // e.g. "https://api.aftertalk.io"
}
```

### 3. `pkg/webhook/client.go` — notification payload (no data)

```go
type NotificationPayload struct {
    SessionID   string    `json:"session_id"`
    RetrieveURL string    `json:"retrieve_url"`   // signed single-use URL
    ExpiresAt   time.Time `json:"expires_at"`
    Timestamp   time.Time `json:"timestamp"`
}

func (c *Client) SendNotification(ctx context.Context, p *NotificationPayload) error {
    body, _ := json.Marshal(p)
    req, _ := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    // HMAC-SHA256 over body with webhook_secret
    if c.secret != "" {
        mac := hmac.New(sha256.New, []byte(c.secret))
        mac.Write(body)
        req.Header.Set("X-Aftertalk-Signature", "hmac-sha256="+hex.EncodeToString(mac.Sum(nil)))
    }
    // ... send
}
```

### 4. `internal/core/minutes/` — token generation and pull endpoint

**Token generation** (in `service.go::deliverWebhook`):

```go
func (s *Service) deliverWebhook(sessionID string, m *Minutes) {
    tok := &RetrievalToken{
        ID:        uuid.New().String(),
        MinutesID: m.ID,
        ExpiresAt: time.Now().Add(s.cfg.TokenTTL),
    }
    s.repo.CreateRetrievalToken(ctx, tok)

    retrieveURL := fmt.Sprintf("%s/v1/minutes/pull/%s", s.cfg.PullBaseURL, tok.ID)
    payload := &webhook.NotificationPayload{
        SessionID:   sessionID,
        RetrieveURL: retrieveURL,
        ExpiresAt:   tok.ExpiresAt,
        Timestamp:   time.Now(),
    }
    // enqueue with retrier or fire-and-forget as before
    s.webhookRetrier.Enqueue(ctx, m.ID, s.webhookClient.URL(), payload)
}
```

**Pull endpoint** (`internal/api/handler/minutes.go`):

```go
// GET /v1/minutes/pull/{token}
// - NO API key required (token is the auth)
// - Validates: token exists, not expired, not used
// - Sets used_at = NOW()
// - Returns minutes JSON
// - If delete_on_pull=true: deletes minutes + transcriptions + token from DB
func (h *MinutesHandler) PullMinutes(w http.ResponseWriter, r *http.Request) {
    tokenID := chi.URLParam(r, "token")
    tok, err := h.minutesService.ConsumeRetrievalToken(r.Context(), tokenID)
    if err != nil {
        // 404 for non-existent/expired/already-used — do not distinguish for security
        http.Error(w, "not found", 404)
        return
    }
    minutes, _ := h.minutesService.GetMinutesByID(r.Context(), tok.MinutesID)
    json.NewEncoder(w).Encode(minutes)

    if h.cfg.DeleteOnPull {
        go h.minutesService.PurgeMinutes(r.Context(), tok.MinutesID)
    }
}
```

**`ConsumeRetrievalToken`** (atomic with transaction):

```go
func (s *Service) ConsumeRetrievalToken(ctx context.Context, tokenID string) (*RetrievalToken, error) {
    return s.repo.ConsumeToken(ctx, tokenID) // UPDATE SET used_at=NOW() WHERE id=? AND used_at IS NULL AND expires_at > NOW()
    // rows affected == 0 → invalid/used/expired
}
```

### 5. Additional route in `server.go`

```go
r.Get("/v1/minutes/pull/{token}", minutesHandler.PullMinutes)
// This route is OUTSIDE the API-key authenticated group
```

---

## Backward compatibility

The `webhook.delete_on_pull` field defaults to `false` → the current (push) behavior is **deprecated but maintained** via feature flag `webhook.mode`:

```yaml
webhook:
  mode: notify_pull   # "push" (legacy) | "notify_pull" (new)
  url: https://...
  secret: "..."
  pull_base_url: https://api.aftertalk.io
  token_ttl: 1h
  delete_on_pull: true
```

---

## SDK (`@aftertalk/sdk`)

Update `MinutesAPI` to handle the pattern on the client side if the caller wants to pull manually:

```typescript
// No changes needed — the pull happens on the recipient server side.
// The SDK client can still use waitForMinutes() which polls GET /v1/sessions/{id}/minutes
// which returns status: 'delivered' when the pull has succeeded.
```

---

## Test plan

- [ ] Expired token → 404 (indistinguishable from not_found)
- [ ] Already-used token → 404
- [ ] Successful pull → minutes deleted from DB (with `delete_on_pull=true`)
- [ ] Successful pull → session status updated to `delivered`
- [ ] HMAC signature on webhook notification is verifiable
- [ ] Webhook retrier works with new `NotificationPayload`
- [ ] `webhook.mode=push` maintains legacy behavior

---

## Priority

**High** — direct push of medical data is incompatible with HIPAA/GDPR in production.
