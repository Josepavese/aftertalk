# 07 — Secure Minutes Delivery: Notify + Pull pattern

## Problema

Il webhook attuale (`pkg/webhook/client.go`) fa un **push diretto** dell'intera minuta strutturata nel body del POST. In un contesto sanitario questo è inaccettabile:

| Problema | Impatto |
|---|---|
| Dati sensibili nel body del webhook | Esposti a ogni intermediario HTTP (proxy, CDN, log) |
| Nessuna firma del payload | Il ricevente non può verificare l'autenticità |
| Minuta non cancellata dopo delivery | Aftertalk diventa un data store medico non intenzionale |
| URL webhook statico lato server | Non configurabile per sessione / tenant |
| Nessuna verifica che il ricevente abbia salvato i dati | Delivery silently lost possibile |

---

## Soluzione: Notify + Pull

Ispirato al pattern Stripe, sistemi HIPAA, DocuSign:

```
Aftertalk                    Recipient server
    │                               │
    │─── POST /webhook ────────────>│  {session_id, retrieve_url, expires_at, sig}
    │    (zero dati sensibili)      │
    │                               │
    │<── GET /v1/minutes/pull/{tok}─│  token single-use HMAC-firmato
    │                               │
    │─── 200 {minutes JSON} ───────>│  trasmissione avviene su connessione TLS del ricevente
    │                               │
    │    [delete from DB] ──────────│  Aftertalk diventa pipeline, non archivio
```

### Garanzie di sicurezza

- **Confidenzialità**: il webhook non trasporta mai dati sensibili, solo un token a scadenza
- **Autenticità**: il webhook ha header `X-Aftertalk-Signature: hmac-sha256=<sig>` su `webhook_secret` configurato — il ricevente verifica prima di fare il pull
- **Non-repudiability**: il pull avviene su connessione TLS del server ricevente, con token monouso
- **Data minimization**: le minute vengono cancellate dal DB Aftertalk dopo pull confermato
- **Replay protection**: token single-use, marcato `used` al primo pull riuscito

---

## Implementazione

### 1. Tabella `retrieval_tokens` (migration inline in `main.go`)

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
    Secret        string        `koanf:"secret"`         // per firma HMAC notification
    TokenTTL      time.Duration `koanf:"token_ttl"`      // default: 1h
    DeleteOnPull  bool          `koanf:"delete_on_pull"` // default: true
    PullBaseURL   string        `koanf:"pull_base_url"`  // es. "https://api.aftertalk.io"
}
```

### 3. `pkg/webhook/client.go` — payload notification (no dati)

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
    // HMAC-SHA256 su body con webhook_secret
    if c.secret != "" {
        mac := hmac.New(sha256.New, []byte(c.secret))
        mac.Write(body)
        req.Header.Set("X-Aftertalk-Signature", "hmac-sha256="+hex.EncodeToString(mac.Sum(nil)))
    }
    // ... send
}
```

### 4. `internal/core/minutes/` — token generation e pull endpoint

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
    // enqueue con retrier o fire-and-forget come ora
    s.webhookRetrier.Enqueue(ctx, m.ID, s.webhookClient.URL(), payload)
}
```

**Pull endpoint** (`internal/api/handler/minutes.go`):

```go
// GET /v1/minutes/pull/{token}
// - NO API key required (il token è l'auth)
// - Verifica: token esiste, non scaduto, non usato
// - Marca used_at = NOW()
// - Ritorna minutes JSON
// - Se delete_on_pull=true: cancella minutes + transcriptions + token dalla DB
func (h *MinutesHandler) PullMinutes(w http.ResponseWriter, r *http.Request) {
    tokenID := chi.URLParam(r, "token")
    tok, err := h.minutesService.ConsumeRetrievalToken(r.Context(), tokenID)
    if err != nil {
        // 404 se non esiste/scaduto/già usato — non distinguere per sicurezza
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

**`ConsumeRetrievalToken`** (atomico con transaction):

```go
func (s *Service) ConsumeRetrievalToken(ctx context.Context, tokenID string) (*RetrievalToken, error) {
    return s.repo.ConsumeToken(ctx, tokenID) // UPDATE SET used_at=NOW() WHERE id=? AND used_at IS NULL AND expires_at > NOW()
    // rows affected == 0 → invalid/used/expired
}
```

### 5. Route aggiuntiva in `server.go`

```go
r.Get("/v1/minutes/pull/{token}", minutesHandler.PullMinutes)
// Questa route è FUORI dal gruppo autenticato da API key
```

---

## Compatibilità backward

Il campo `webhook.delete_on_pull` è `false` di default → il comportamento attuale (push) viene **deprecato ma mantenuto** tramite feature flag `webhook.mode`:

```yaml
webhook:
  mode: notify_pull   # "push" (legacy) | "notify_pull" (nuovo)
  url: https://...
  secret: "..."
  pull_base_url: https://api.aftertalk.io
  token_ttl: 1h
  delete_on_pull: true
```

---

## SDK (`@aftertalk/sdk`)

Aggiornare `MinutesAPI` per gestire il pattern lato client se il chiamante vuole fare il pull manualmente:

```typescript
// Nessuna modifica necessaria — il pull avviene lato server del ricevente.
// Il client SDK può comunque usare waitForMinutes() che fa polling su GET /v1/sessions/{id}/minutes
// che ritorna status: 'delivered' quando il pull è avvenuto con successo.
```

---

## Test plan

- [ ] Token scaduto → 404 (indistinguibile da not_found)
- [ ] Token già usato → 404
- [ ] Pull riuscito → minuta cancellata dal DB (con `delete_on_pull=true`)
- [ ] Pull riuscito → status sessione aggiornato a `delivered`
- [ ] Firma HMAC sul webhook notification verificabile
- [ ] Retrier webhook funziona con il nuovo payload `NotificationPayload`
- [ ] `webhook.mode=push` mantiene comportamento legacy

---

## Priorità

**Alta** — il push diretto di dati medici è incompatibile con HIPAA/GDPR in produzione.
