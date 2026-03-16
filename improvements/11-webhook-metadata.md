# Improvement 11: Metadata nei Payload Webhook

## Stato: APERTO

## Contesto

Il campo `Session.Metadata` (tipo `string`, valore JSON opaco) esiste già sia nell'entity Go
(`internal/core/session/entity.go:21`) sia nel request DTO (`internal/api/handler/session.go:46`).
È persistito in SQLite e viene passato al momento della creazione della sessione.

Il problema è che questo campo non viene mai **propagato al payload webhook**.
Quando Aftertalk consegna le minute via webhook, il ricevente ottiene:

```json
{
  "session_id": "sess_abc",
  "timestamp": "2026-03-16T10:00:00Z",
  "minutes": { "sections": {...}, "citations": [...] }
}
```

Non c'è modo per il destinatario di sapere a quale contesto applicativo (appuntamento, utente,
paziente) appartiene quella sessione, se non mantenendo in proprio una tabella di mapping
`session_id → contesto`. Questo è ridondante e fragile.

### Caso d'uso che ha evidenziato il problema

MondoPsicologi vuole integrare Aftertalk nel proprio backend PHP. Al momento della creazione della
sessione, il backend PHP manda metadata come:

```json
{
  "appointment_id": "appt_123",
  "doctor_id": "doc_456",
  "patient_id": "pat_789",
  "organization": "mondopsicologi"
}
```

Quando arriva il webhook con le minute, il backend PHP deve associarle all'appuntamento corretto
per notificare il dottore. Senza metadata nel payload, l'unica opzione è fare una query:
`SELECT appointment_id FROM appointment_calls WHERE aftertalk_session_id = ?` — che è una
dipendenza di stato locale fragile e introduce coupling non necessario.

### Cosa già funziona

- `Session.Metadata` è già letto dalla richiesta di creazione.
- È già salvato nel DB (colonna `metadata TEXT` nella tabella `sessions`).
- `session.Service.GenerateMinutesForSession` carica la sessione prima di generare le minute.

### Cosa manca

1. `MinutesPayload` non include `SessionMetadata`.
2. `NotificationPayload` non include `SessionMetadata`.
3. `deliverWebhook` in `internal/core/minutes/service.go` non legge i metadata dalla sessione
   prima di costruire il payload.
4. Opzionale ma utile: includere un sommario dei partecipanti (user_id + role) per sapere chi
   erano dottore e paziente senza dover fare una seconda chiamata API.

---

## Implementazione

### 1. Estendere `MinutesPayload` e `NotificationPayload`

File: `pkg/webhook/client.go`

```go
type ParticipantSummary struct {
    UserID string `json:"user_id"`
    Role   string `json:"role"`
}

type MinutesPayload struct {
    Timestamp       time.Time            `json:"timestamp"`
    Minutes         interface{}          `json:"minutes"`
    SessionID       string               `json:"session_id"`
    SessionMetadata string               `json:"session_metadata,omitempty"` // JSON opaco passato alla creazione
    Participants    []ParticipantSummary `json:"participants,omitempty"`
}

type NotificationPayload struct {
    ExpiresAt       time.Time `json:"expires_at"`
    Timestamp       time.Time `json:"timestamp"`
    SessionID       string    `json:"session_id"`
    RetrieveURL     string    `json:"retrieve_url"`
    SessionMetadata string    `json:"session_metadata,omitempty"`
}
```

### 2. Aggiornare `deliverWebhook` e `deliverNotification`

File: `internal/core/minutes/service.go`

La funzione `deliverWebhook` deve ricevere (o caricare) i metadata della sessione e i partecipanti
prima di costruire il payload. Il `minutes.Service` ha già accesso a `session.Repository` tramite
il service di sessione — oppure `session.Metadata` può essere passato come parametro da
`generateMinutesForSession` che già carica la sessione.

### 3. Aggiornare `PullMinutes` handler

File: `internal/api/handler/minutes.go`

Il handler `PullMinutes` (notify_pull) deve anch'esso includere i metadata nel body della risposta
JSON, in modo che il recipient ottenga il contesto completo anche quando usa il pattern pull.

---

## Compatibilità

La modifica è **backward-compatible**: i nuovi campi usano `omitempty`. I riceventi esistenti
che ignorano campi sconosciuti non sono impattati. I nuovi riceventi possono leggere i metadata
senza cambiare versione API.

---

## Documentazione da aggiornare

Quando questa feature è implementata, aggiornare obbligatoriamente:

- **`docs/wiki/webhook-delivery.md`** — aggiungere descrizione dei campi `session_metadata` e
  `participants` in entrambi i payload, con esempio JSON completo.
- **`docs/wiki/integration-guide.md`** (da creare se non esiste) — spiegare il pattern
  "crea sessione con metadata → ricevi webhook con metadata" end-to-end.
- **`pkg/webhook/client.go`** — aggiornare il commento del package con il nuovo schema.
- **`README.md`** — aggiornare la sezione webhook con i nuovi campi.
- **SDK JS/TS** (`sdk/types.ts`) — aggiornare i tipi `MinutesPayload` e `NotificationPayload`.
- **SDK PHP** (quando creato) — aggiornare `WebhookPayload` con i nuovi campi.
