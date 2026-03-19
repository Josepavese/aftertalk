# Improvement 22: Go SDK (`aftertalk/aftertalk-go`)

## Stato: APERTO

## Contesto

Aftertalk dispone già di un SDK TypeScript (`@aftertalk/sdk`, improvement 04) per il lato
frontend, e di un SDK PHP pianificato (`aftertalk/aftertalk-php`, improvement 13) per i
backend PHP. Manca un SDK per applicazioni **backend Go** che devono integrarsi con Aftertalk
via HTTP — scenario frequente per servizi interni, microservizi, o piattaforme Go-native.

> **Distinzione fondamentale**: questo SDK non è il core Aftertalk (il motore Go che trascrisce
> e genera minute). E' un **client HTTP** per applicazioni Go esterne che vogliono creare
> sessioni, generare token, terminare sessioni, e ricevere/verificare webhook — esattamente
> come fa l'SDK PHP ma in Go.

Il caso d'uso primario è un'applicazione server Go (es. gateway di prenotazioni, backend
MondoPsicologi scritto in Go, servizio di orchestrazione) che deve:

1. Creare una sessione Aftertalk con metadata, `template_id`, `stt_profile`, `llm_profile`
2. Generare token JWT partecipanti da restituire ai client frontend
3. Terminare la sessione a fine chiamata
4. Ricevere e verificare payload webhook firmati HMAC-SHA256
5. Opzionalmente: fare polling delle minute fino allo stato `ready`

### Analogia con gli altri SDK

| Responsabilità | SDK TS (`@aftertalk/sdk`) | SDK PHP (`aftertalk/aftertalk-php`) | SDK Go (`aftertalk/aftertalk-go`) |
|---|---|---|---|
| Creare sessioni | SessionsAPI | SessionsApi | SessionsClient |
| Generare token partecipanti | SessionsAPI | SessionsApi | SessionsClient |
| Terminare sessione | SessionsAPI | SessionsApi | SessionsClient |
| Connessione WebRTC | WebRTCConnection | non applicabile | non applicabile |
| Polling minute | MinutesPoller | opzionale | opzionale |
| Verificare webhook HMAC | non applicabile | WebhookHandler | WebhookHandler |
| Deserializzare payload webhook | non applicabile | WebhookHandler | WebhookHandler |
| Selezione profilo STT/LLM | tramite API | tramite API | tramite API |

---

## Struttura del repository

```
aftertalk-go/
├── go.mod                        # module aftertalk/aftertalk-go, Go 1.22
├── go.sum
├── README.md
├── aftertalk.go                  # package aftertalk — Client (entry point pubblico)
├── config.go                     # ClientConfig, opzioni funzionali
├── errors.go                     # AftertalkError, codici errore
├── sessions.go                   # SessionsClient: CreateSession, EndSession, GetSession
├── minutes.go                    # MinutesClient: GetMinutes, UpdateMinutes, GetVersions
├── transcriptions.go             # TranscriptionsClient: GetTranscriptions
├── webhook.go                    # WebhookHandler: VerifySignature, ParsePayload
├── poller.go                     # MinutesPoller: PollUntilReady (con exponential backoff)
├── types.go                      # DTO: Session, Minutes, Participant, Transcription, ecc.
├── http.go                       # httpClient interno (net/http, retry, header API key)
└── webhook_test.go               # unit test verifica firma HMAC
    sessions_test.go              # unit test con httptest.Server
    minutes_test.go
    poller_test.go
```

---

## API pubblica principale

### Inizializzazione client

```go
import aftertalk "aftertalk/aftertalk-go"

client, err := aftertalk.New(
    aftertalk.WithBaseURL("https://aftertalk.yourserver.com"),
    aftertalk.WithAPIKey(os.Getenv("AFTERTALK_API_KEY")),
    aftertalk.WithWebhookSecret(os.Getenv("AFTERTALK_WEBHOOK_SECRET")),
    // opzionale:
    aftertalk.WithTimeout(30 * time.Second),
    aftertalk.WithHTTPClient(myCustomHTTPClient),
)
if err != nil {
    log.Fatal(err)
}
```

### Creazione sessione

```go
// POST /v1/sessions
session, err := client.Sessions.Create(ctx, aftertalk.CreateSessionRequest{
    TemplateID:       "therapy",
    ParticipantCount: 2,
    STTProfile:       "cloud",   // opzionale — usa default se omesso
    LLMProfile:       "local",   // opzionale
    Metadata: map[string]string{
        "appointment_id": "appt_123",
        "doctor_id":      "doc_456",
        "patient_id":     "pat_789",
    },
})
if err != nil {
    return fmt.Errorf("create session: %w", err)
}
// session.ID, session.TemplateID, session.CreatedAt

// Partecipanti: i token sono inclusi nella risposta
for _, p := range session.Participants {
    // p.Token — JWT monouso da restituire al frontend per /signaling
    // p.UserID, p.Role
}
```

### Fine sessione

```go
// POST /v1/sessions/{id}/end
if err := client.Sessions.End(ctx, session.ID); err != nil {
    return fmt.Errorf("end session: %w", err)
}
```

### Recupero minute

```go
// GET /v1/sessions/{id}/minutes
minutes, err := client.Minutes.Get(ctx, session.ID)
if err != nil {
    return fmt.Errorf("get minutes: %w", err)
}
// minutes.Status: "pending" | "ready" | "delivered" | "error"
// minutes.Sections: map[string]json.RawMessage  (sezioni dinamiche per template)
// minutes.Citations: []Citation{Text, Role, TimestampMs}
// minutes.TemplateID, minutes.Version
```

### Polling minute con backoff

```go
// Attende fino a che le minute sono in stato "ready" o "delivered"
minutes, err := client.Minutes.PollUntilReady(ctx, session.ID, aftertalk.PollerOptions{
    Timeout:     2 * time.Minute,
    MinInterval: 3 * time.Second,
    MaxInterval: 30 * time.Second,
})
if err != nil {
    // aftertalk.ErrPollingTimeout oppure aftertalk.ErrMinutesGenerationFailed
    return fmt.Errorf("poll minutes: %w", err)
}
```

### Aggiornamento minuta

```go
// PUT /v1/sessions/{id}/minutes
updated, err := client.Minutes.Update(ctx, session.ID, aftertalk.UpdateMinutesRequest{
    Sections: map[string]json.RawMessage{
        "themes": json.RawMessage(`["Ansia da prestazione","Relazione genitori"]`),
    },
})
```

### Trascrizioni

```go
// GET /v1/sessions/{id}/transcriptions
transcriptions, err := client.Transcriptions.List(ctx, session.ID)
for _, t := range transcriptions {
    // t.Role, t.Text, t.StartedAtMs, t.EndedAtMs, t.Confidence
}
```

### Verifica e parsing webhook

```go
// Handler HTTP (es. net/http, chi, gin, fiber...)
func webhookHandler(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    sig := r.Header.Get("X-Aftertalk-Signature")

    if err := client.Webhook.VerifySignature(body, sig); err != nil {
        // aftertalk.ErrWebhookSignatureInvalid
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    payload, err := client.Webhook.ParsePayload(body)
    if err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }

    switch p := payload.(type) {
    case *aftertalk.MinutesPayload:
        // Webhook mode "push" — payload completo
        // p.SessionID, p.SessionMetadata, p.Minutes, p.Participants
        appointmentID := p.SessionMetadata["appointment_id"]
        _ = saveMinutes(appointmentID, p.Minutes)

    case *aftertalk.NotificationPayload:
        // Webhook mode "notify_pull" — URL per il recupero
        // p.SessionID, p.RetrieveURL, p.ExpiresAt
        go func() {
            full, _ := client.Webhook.PullMinutes(ctx, p.RetrieveURL)
            _ = saveMinutes(full.SessionMetadata["appointment_id"], full.Minutes)
        }()
    }

    w.WriteHeader(http.StatusOK)
}
```

---

## Tipi principali (`types.go`)

```go
// Session rappresenta una sessione Aftertalk
type Session struct {
    ID           string            `json:"session_id"`
    Status       SessionStatus     `json:"status"`
    TemplateID   string            `json:"template_id,omitempty"`
    STTProfile   string            `json:"stt_profile,omitempty"`
    LLMProfile   string            `json:"llm_profile,omitempty"`
    Participants []Participant     `json:"participants"`
    Metadata     map[string]string `json:"metadata,omitempty"`
    CreatedAt    time.Time         `json:"created_at"`
    EndedAt      *time.Time        `json:"ended_at,omitempty"`
}

type SessionStatus string
const (
    SessionActive     SessionStatus = "active"
    SessionEnded      SessionStatus = "ended"
    SessionProcessing SessionStatus = "processing"
    SessionCompleted  SessionStatus = "completed"
    SessionError      SessionStatus = "error"
)

type Participant struct {
    ParticipantID string     `json:"participant_id"`
    UserID        string     `json:"user_id"`
    Role          string     `json:"role"`
    Token         string     `json:"token"`            // JWT monouso per /signaling
    ConnectedAt   *time.Time `json:"connected_at,omitempty"`
}

type Minutes struct {
    ID         string                     `json:"id"`
    SessionID  string                     `json:"session_id"`
    TemplateID string                     `json:"template_id"`
    Status     MinutesStatus              `json:"status"`
    Sections   map[string]json.RawMessage `json:"sections"`
    Citations  []Citation                 `json:"citations"`
    Version    int                        `json:"version"`
    Provider   string                     `json:"provider,omitempty"`
    GeneratedAt *time.Time                `json:"generated_at,omitempty"`
}

type MinutesStatus string
const (
    MinutesPending   MinutesStatus = "pending"
    MinutesReady     MinutesStatus = "ready"
    MinutesDelivered MinutesStatus = "delivered"
    MinutesError     MinutesStatus = "error"
)

type Citation struct {
    Text        string `json:"text"`
    Role        string `json:"role"`
    TimestampMs int64  `json:"timestamp_ms"`
}

type Transcription struct {
    ID          string              `json:"id"`
    SessionID   string              `json:"session_id"`
    Role        string              `json:"role"`
    Text        string              `json:"text"`
    Status      TranscriptionStatus `json:"status"`
    Confidence  float64             `json:"confidence"`
    StartedAtMs int64               `json:"started_at_ms"`
    EndedAtMs   int64               `json:"ended_at_ms"`
    CreatedAt   time.Time           `json:"created_at"`
}

// MinutesPayload — webhook mode "push"
type MinutesPayload struct {
    SessionID       string            `json:"session_id"`
    SessionMetadata map[string]string `json:"session_metadata"`
    Minutes         Minutes           `json:"minutes"`
    Participants    []ParticipantSummary `json:"participants"`
    Timestamp       time.Time         `json:"timestamp"`
}

// NotificationPayload — webhook mode "notify_pull"
type NotificationPayload struct {
    SessionID   string    `json:"session_id"`
    RetrieveURL string    `json:"retrieve_url"`
    ExpiresAt   time.Time `json:"expires_at"`
    Timestamp   time.Time `json:"timestamp"`
}
```

---

## Gestione errori

```go
// errors.go
var (
    ErrUnauthorized              = &AftertalkError{Code: "unauthorized"}
    ErrNotFound                  = &AftertalkError{Code: "not_found"}
    ErrSessionNotFound           = &AftertalkError{Code: "session_not_found"}
    ErrWebhookSignatureInvalid   = &AftertalkError{Code: "webhook_signature_invalid"}
    ErrPollingTimeout            = &AftertalkError{Code: "polling_timeout"}
    ErrMinutesGenerationFailed   = &AftertalkError{Code: "minutes_generation_failed"}
)

type AftertalkError struct {
    Code       string `json:"code"`
    Message    string `json:"message"`
    StatusCode int    `json:"-"`
}

func (e *AftertalkError) Error() string {
    if e.Message != "" {
        return fmt.Sprintf("aftertalk: %s: %s", e.Code, e.Message)
    }
    return fmt.Sprintf("aftertalk: %s", e.Code)
}

// Il chiamante può fare type assertion o usare errors.Is/errors.As:
var aftErr *aftertalk.AftertalkError
if errors.As(err, &aftErr) && aftErr.StatusCode == 404 {
    // sessione non trovata
}
```

---

## Pattern get-or-create (anti race condition)

Come per il PHP SDK, la logica get-or-create appartiene al chiamante. La documentazione
deve mostrare il pattern idiomatico Go con transazione DB:

```go
func getOrCreateSession(ctx context.Context, db *sql.DB, client *aftertalk.Client,
    appointmentID string, appt Appointment) (string, error) {

    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return "", err
    }
    defer tx.Rollback()

    var sessionID sql.NullString
    err = tx.QueryRowContext(ctx,
        `INSERT INTO appointment_calls (appointment_id) VALUES (?)
         ON CONFLICT (appointment_id) DO UPDATE SET appointment_id = excluded.appointment_id
         RETURNING aftertalk_session_id`,
        appointmentID,
    ).Scan(&sessionID)
    if err != nil {
        return "", err
    }

    if sessionID.Valid {
        return sessionID.String, tx.Commit()
    }

    session, err := client.Sessions.Create(ctx, aftertalk.CreateSessionRequest{
        TemplateID: "therapy",
        STTProfile: appt.STTProfile,
        LLMProfile: appt.LLMProfile,
        Metadata: map[string]string{
            "appointment_id": appointmentID,
            "doctor_id":      appt.DoctorID,
            "patient_id":     appt.PatientID,
        },
    })
    if err != nil {
        return "", fmt.Errorf("create aftertalk session: %w", err)
    }

    _, err = tx.ExecContext(ctx,
        `UPDATE appointment_calls SET aftertalk_session_id = ? WHERE appointment_id = ?`,
        session.ID, appointmentID,
    )
    if err != nil {
        return "", err
    }

    return session.ID, tx.Commit()
}
```

---

## Dipendenze

Il SDK Go privilegia la libreria standard, senza dipendenze opzionali su framework HTTP.

| Dipendenza | Motivo | Alternativa |
|---|---|---|
| `net/http` (stdlib) | HTTP client | nessuna — zero deps |
| `crypto/hmac` + `crypto/sha256` (stdlib) | Verifica firma webhook | nessuna |
| `encoding/json` (stdlib) | Serializzazione | nessuna |
| `context` (stdlib) | Propagazione cancellazione | nessuna |

**Nessuna dipendenza esterna obbligatoria.** Il `go.mod` dichiara solo `go 1.22`.

Opzionale (solo per testing):
- `github.com/stretchr/testify` — assert nei test (dev dependency)

---

## Confronto tra i tre SDK

| Caratteristica | `@aftertalk/sdk` (TS) | `aftertalk-php` (PHP) | `aftertalk-go` (Go) |
|---|---|---|---|
| Target primario | Frontend browser | Backend PHP (Laravel/Symfony) | Backend Go (microservizi) |
| Connessione WebRTC | Si (WebRTCConnection) | No | No |
| Verifica webhook HMAC | No (frontend) | Si | Si |
| Polling minute | Si (MinutesPoller) | Opzionale | Si (MinutesPoller) |
| Profili STT/LLM | Tramite API | Tramite API | Tramite API |
| Gestione errori | Gerarchia AftertalkError | Gerarchia AftertalkException | Sentinel errors + AftertalkError |
| Dipendenze runtime | Zero (no node_modules runtime) | PSR-18 / Guzzle | Zero (stdlib only) |
| Package manager | npm / `@aftertalk/sdk` | Composer / Packagist | `go get aftertalk/aftertalk-go` |
| Tipizzazione | TypeScript strong | PHP 8.1 readonly/enum | Go strong static |
| Context/cancellation | Promise/AbortSignal | timeout Guzzle | `context.Context` nativo |
| Test | Vitest | PHPUnit | `testing` stdlib + testify |

---

## Verifica firma webhook — dettaglio implementazione

```go
// webhook.go

// VerifySignature verifica la firma HMAC-SHA256 del body ricevuto.
// Il formato atteso dell'header è: "sha256=<hex-digest>"
func (h *WebhookHandler) VerifySignature(body []byte, signatureHeader string) error {
    if !strings.HasPrefix(signatureHeader, "sha256=") {
        return ErrWebhookSignatureInvalid
    }
    expected := signatureHeader[len("sha256="):]

    mac := hmac.New(sha256.New, []byte(h.secret))
    mac.Write(body)
    actual := hex.EncodeToString(mac.Sum(nil))

    if !hmac.Equal([]byte(actual), []byte(expected)) {
        return ErrWebhookSignatureInvalid
    }
    return nil
}

// ParsePayload deserializza il body del webhook nel tipo corretto
// (MinutesPayload o NotificationPayload) in base al campo "type".
func (h *WebhookHandler) ParsePayload(body []byte) (interface{}, error) {
    var envelope struct {
        Type string `json:"type"`
    }
    if err := json.Unmarshal(body, &envelope); err != nil {
        return nil, fmt.Errorf("parse webhook envelope: %w", err)
    }

    switch envelope.Type {
    case "minutes":
        var p MinutesPayload
        if err := json.Unmarshal(body, &p); err != nil {
            return nil, err
        }
        return &p, nil
    case "notification":
        var p NotificationPayload
        if err := json.Unmarshal(body, &p); err != nil {
            return nil, err
        }
        return &p, nil
    default:
        return nil, fmt.Errorf("unknown webhook type: %s", envelope.Type)
    }
}
```

---

## Integrazione con framework HTTP Go

Il SDK non impone nessun framework. Esempi di integrazione:

### net/http (stdlib)

```go
mux := http.NewServeMux()
mux.HandleFunc("POST /webhooks/aftertalk", webhookHandler)
```

### chi

```go
r := chi.NewRouter()
r.Post("/webhooks/aftertalk", webhookHandler)
```

### gin

```go
r := gin.Default()
r.POST("/webhooks/aftertalk", func(c *gin.Context) {
    body, _ := io.ReadAll(c.Request.Body)
    if err := client.Webhook.VerifySignature(body, c.GetHeader("X-Aftertalk-Signature")); err != nil {
        c.Status(http.StatusUnauthorized)
        return
    }
    // ...
})
```

---

## Testing

```go
// sessions_test.go — mock con httptest
func TestSessionsCreate(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "POST", r.Method)
        assert.Equal(t, "/v1/sessions", r.URL.Path)
        assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(Session{ID: "sess_123", Status: SessionActive})
    }))
    defer srv.Close()

    client, _ := aftertalk.New(
        aftertalk.WithBaseURL(srv.URL),
        aftertalk.WithAPIKey("test-key"),
    )
    session, err := client.Sessions.Create(context.Background(), CreateSessionRequest{
        TemplateID: "therapy",
    })

    require.NoError(t, err)
    assert.Equal(t, "sess_123", session.ID)
}

// webhook_test.go — verifica firma HMAC senza HTTP
func TestWebhookVerifySignature(t *testing.T) {
    secret := "test-secret"
    body := []byte(`{"type":"minutes","session_id":"sess_123"}`)

    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

    client, _ := aftertalk.New(
        aftertalk.WithBaseURL("http://localhost"),
        aftertalk.WithWebhookSecret(secret),
    )
    err := client.Webhook.VerifySignature(body, sig)
    require.NoError(t, err)

    // firma errata
    err = client.Webhook.VerifySignature(body, "sha256=invalidsig")
    assert.ErrorIs(t, err, aftertalk.ErrWebhookSignatureInvalid)
}
```

---

## Priorità di implementazione

| # | Task | Effort | Priorita |
|---|---|---|---|
| 1 | `types.go` — DTO allineati alle API Aftertalk | Basso (2h) | Alta |
| 2 | `http.go` — client interno con retry, header, timeout | Medio (3h) | Alta |
| 3 | `sessions.go` — Create, End, Get | Basso (2h) | Alta |
| 4 | `webhook.go` — VerifySignature, ParsePayload, PullMinutes | Basso (2h) | Alta |
| 5 | `minutes.go` — Get, Update, GetVersions | Basso (2h) | Media |
| 6 | `transcriptions.go` — List | Basso (1h) | Media |
| 7 | `poller.go` — PollUntilReady con backoff esponenziale | Basso (2h) | Media |
| 8 | Test unitari per tutti i moduli | Medio (4h) | Alta |
| 9 | README + godoc | Medio (3h) | Media |
| 10 | Pubblicazione su GitHub + `go get` | Basso (1h) | Bassa |

---

## Documentazione da aggiornare

Quando l'SDK Go e' rilasciato, aggiornare obbligatoriamente:

- **`README.md` (root Aftertalk)** — aggiungere nella sezione SDK il link a `aftertalk-go`
  accanto a `@aftertalk/sdk` e `aftertalk-php`.
- **`docs/wiki/sdks.md`** (o equivalente) — aggiungere pagina dedicata Go SDK con:
  installazione (`go get`), quickstart, tabella metodi, gestione webhook, pattern
  get-or-create, esempi chi/gin/fiber.
- **`docs/wiki/integration-guide.md`** — sezione "Integrazione backend Go" con flusso
  completo (appointment → session → token → webhook).
- **`aftertalk-go/README.md`** — README del repository Go: installazione, quickstart,
  tutti i metodi, esempi webhook, sicurezza (non esporre API key al frontend).
- **`@aftertalk/sdk` README** — aggiungere nota che per backend Go esiste `aftertalk/aftertalk-go`.
- **`aftertalk-php/README.md`** (quando pubblicato) — riferimento incrociato all'SDK Go.
