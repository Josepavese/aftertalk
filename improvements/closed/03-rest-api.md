# Improvement: REST API — Moderna, Sicura, Organizzata

## Verdetto Avvocato del Diavolo

**L'asserzione "REST API ben organizzate, moderne, semplici e sicure" è PARZIALMENTE vera.**

La struttura di base è corretta (chi router, v1 prefix, handler separati, OpenAPI spec). Ma mancano componenti critici per la sicurezza e la completezza produttiva.

---

## Gaps Identificati

### 1. CORS Wildcard — Insicuro in Produzione

**Problema**: CORS è configurato con `Access-Control-Allow-Origin: *` — accetta richieste da qualsiasi origine.

```go
// internal/api/middleware/middleware.go (presumibilmente)
// o server.go:101
cors.Handler(cors.Options{
    AllowedOrigins: []string{"*"},  // ← INSICURO
    AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders: []string{"*"},
})
```

**Impatto**: In produzione, qualsiasi sito web può fare richieste autenticate all'API se l'utente ha un API key valida nel browser. Abilita attacchi CSRF cross-origin.

**Fix Richiesto**:
```yaml
# config.yaml
api:
  cors:
    allowed_origins:
      - "https://app.miosito.com"
      - "http://localhost:3000"  # dev only
    allow_credentials: true
```

```go
// server.go — CORS configurabile
cors.Handler(cors.Options{
    AllowedOrigins: cfg.API.CORS.AllowedOrigins,
    AllowCredentials: cfg.API.CORS.AllowCredentials,
})
```

---

### 2. Nessun Rate Limiting

**Problema**: Non esiste nessuna protezione contro:
- Brute force sull'API key
- DoS via molte richieste simultanee
- Abuso del POST /test/start (crea sessioni illimitate)

Manca completamente qualsiasi forma di throttling.

**Fix Richiesto**:
```go
// Aggiungere middleware rate limiter (golang.org/x/time/rate o go-chi/httprate)
import "github.com/go-chi/httprate"

r.Use(httprate.LimitByIP(100, 1*time.Minute))  // 100 req/min per IP

// Rate limite specifico per endpoint costosi
r.With(httprate.LimitByIP(10, 1*time.Minute)).Post("/v1/sessions", ...)
```

---

### 3. `/demo/config` Espone API Key — Vulnerabilità Critica

**Problema**: L'endpoint `/demo/config` è pubblico (no auth) e restituisce l'API key del server.

```go
// internal/api/server.go
r.Get("/demo/config", func(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]interface{}{
        "api_key":             cfg.API.Key,  // ← API KEY ESPOSTA PUBBLICAMENTE
        "templates":           cfg.Templates,
        "default_template_id": defaultTemplateID,
    })
})
```

**Impatto**: Chiunque possa accedere all'URL del server può ottenere l'API key e fare chiamate autenticate all'intera API. Questo è accettabile in demo locale, **non in produzione**.

**Fix Richiesto**:
```go
// Separare i due concern:

// 1. Endpoint pubblico — solo metadata pubblici
r.Get("/v1/config", func(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]interface{}{
        "templates":           cfg.Templates,
        "default_template_id": defaultTemplateID,
        // NO api_key
    })
})

// 2. Config demo locale — solo se DEMO_MODE=true (env var)
if cfg.Demo.Enabled {
    r.Get("/demo/config", func(...) {
        // Può includere api_key solo in modalità demo esplicita
    })
}
```

---

### 4. `/test/start` Non Protetto — Può Creare Sessioni Arbitrarie

**Problema**: `POST /test/start` crea sessioni senza autenticazione. Chiunque può creare sessioni sul server.

```go
// Endpoint pubblico che crea sessioni:
r.Post("/test/start", func(...) {
    // Crea session + participants + token
    // Nessuna auth richiesta
})
```

**Fix Richiesto**: Questo endpoint dovrebbe o:
1. Richiedere autenticazione (API key)
2. Essere rimosso dalla produzione e disponibile solo con `--dev` flag
3. Avere un limite di sessioni attive per IP

---

### 5. Nessuna Input Validation Strutturata

**Problema**: La validazione degli input è ad-hoc, inconsistente tra gli handler.

**Esempi di validazione mancante**:

```go
// handler/session.go — presumibilmente:
// Non verifica che participant_count corrisponda a len(participants)
// Non verifica che i roles siano validi per il template scelto
// Non verifica che user_id non sia vuoto
// Non ha limiti sulla lunghezza di stringhe (user_id da 1MB?)
```

```go
// handler/minutes.go:
// PUT /v1/minutes/{id} — accetta qualsiasi JSON, nessuna validazione del formato sections
```

**Fix Richiesto**: Validazione centralizzata con libreria:
```go
// Opzione: github.com/go-playground/validator/v10
type CreateSessionRequest struct {
    ParticipantCount int           `json:"participant_count" validate:"required,min=2,max=10"`
    TemplateID       string        `json:"template_id"       validate:"omitempty,max=64"`
    Participants     []Participant `json:"participants"       validate:"required,min=2,max=10,dive"`
}

type Participant struct {
    UserID string `json:"user_id" validate:"required,min=1,max=128"`
    Role   string `json:"role"    validate:"required,min=1,max=64"`
}
```

---

### 6. Formato Errori Inconsistente

**Problema**: Gli handler restituiscono errori in formati diversi:

```go
// Alcuni handler:
http.Error(w, "Session ID required", http.StatusBadRequest)
// → plain text, non JSON

// Altri:
json.NewEncoder(w).Encode(map[string]string{"error": msg})
// → JSON

// La spec OpenAPI dichiara:
// ErrorResponse: {error: string}
// Ma alcuni endpoint restituiscono plain text
```

**Fix Richiesto**: Centralizzare il formato errore:
```go
// internal/api/response/response.go
func Error(w http.ResponseWriter, status int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Tutti gli handler usano:
response.Error(w, http.StatusBadRequest, "Session ID required")
```

---

### 7. OpenAPI Spec Non Sincronizzata con il Codice

**Problema**: Esiste `specs/contracts/api.yaml` (OpenAPI 3.0.3) ma:
- Non è generata dal codice, è scritta a mano
- Può andare out-of-sync con gli handler reali
- Non esiste nessun test che validi la conformità
- I server dichiarati puntano a `api.aftertalk.io` ma il server gira su porta 8080

```yaml
# specs/contracts/api.yaml
servers:
  - url: https://api.aftertalk.io/v1    # ← dominio inesistente
  - url: http://localhost:3000/v1        # ← porta sbagliata (usa 8080)
```

**Fix Richiesto**:
1. Serving automatico della spec: `GET /v1/openapi.yaml` → serve il file
2. Swagger UI embedded: `GET /docs` → interfaccia interattiva
3. Test di conformità: `go test` verifica che ogni route dichiarata nella spec esista nel router
4. Generazione automatica (lungo termine): `swaggo/swag` o `ogen/ogen`

---

### 8. Nessun Endpoint per Listing Sessions

**Problema**: Non esiste `GET /v1/sessions` (lista sessioni). Un'applicazione client non può mostrare lo storico delle sessioni.

**Endpoints mancanti nell'API**:
| Endpoint | Uso | Priorità |
|---|---|---|
| `GET /v1/sessions` | Lista sessioni (con filtri, paginazione) | Alta |
| `GET /v1/sessions/{id}/status` | Polling status elaborazione | Media |
| `DELETE /v1/sessions/{id}` | Cancellazione sessione (GDPR) | Alta |
| `GET /v1/transcriptions/{id}` | Singola trascrizione per ID | Media |
| `DELETE /v1/minutes/{id}` | Cancellazione minuta (GDPR) | Media |

---

### 9. Nessuna Paginazione

**Problema**: `GET /v1/transcriptions?session_id=...` restituisce tutte le trascrizioni senza limite. Una sessione lunga può avere centinaia di segmenti → risposta potenzialmente huge.

**Fix Richiesto**:
```
GET /v1/transcriptions?session_id=xxx&limit=50&offset=0
GET /v1/sessions?status=completed&limit=20&page=2&sort=created_at:desc
```

---

### 10. Nessun Meccanismo di Autenticazione Multi-Tenant

**Problema**: L'API ha un singolo API key globale. Non è possibile:
- Avere più applicazioni client con chiavi diverse
- Revocare una singola chiave
- Tracciare quale client ha fatto quale richiesta

**Fix Richiesto**: API key management:
```
POST /v1/api-keys        → Crea nuova API key (richiede master key)
GET  /v1/api-keys        → Lista API keys
DELETE /v1/api-keys/{id} → Revoca API key
```

---

## Matrice di Conformità "Moderna e Sicura"

| Caratteristica | Stato | Note |
|---|---|---|
| Versionamento (`/v1/`) | ✅ Presente | — |
| Bearer Token Auth | ✅ Presente | Ma singola chiave globale |
| JWT per WebRTC | ✅ Presente | — |
| CORS | ⚠️ Wildcard | Insicuro in produzione |
| Rate Limiting | ❌ Assente | — |
| Input Validation | ⚠️ Parziale | Ad-hoc, inconsistente |
| Error Format Consistente | ⚠️ Parziale | Mix JSON/plain text |
| OpenAPI Spec | ⚠️ Presente ma stale | Non sincronizzata, server sbagliati |
| Swagger UI | ❌ Assente | — |
| Paginazione | ❌ Assente | — |
| `/demo/config` sicuro | ❌ Espone API key | Critico |
| `/test/start` protetto | ❌ Pubblico | Crea sessioni senza auth |
| GDPR endpoints | ❌ Assenti | No DELETE session/minutes |
| Logging richieste | ✅ Presente | Zap middleware |
| Request ID | ✅ Presente | `X-Request-ID` |
| Timeout globale | ✅ Presente | 60s |
| Recovery (panic) | ✅ Presente | — |

---

## Priorità di Intervento

| # | Gap | Impatto | Effort | Priorità |
|---|-----|---------|--------|----------|
| 3 | `/demo/config` espone API key | Critico | Basso | **Critica** |
| 4 | `/test/start` non protetto | Alto | Basso | **Alta** |
| 2 | Nessun rate limiting | Alto | Medio | **Alta** |
| 6 | Errori inconsistenti (già package presente) | Medio | Basso | **Alta** |
| 1 | CORS wildcard | Alto | Basso | **Alta** |
| 5 | Input validation | Medio | Medio | **Media** |
| 8 | Endpoints mancanti (listing, delete) | Medio | Medio | **Media** |
| 7 | OpenAPI stale + no Swagger UI | Basso | Medio | **Media** |
| 9 | Nessuna paginazione | Medio | Medio | **Media** |
| 10 | Single API key | Basso | Alto | **Bassa** |

---

## Passi di Implementazione

### Step 1 — Fix Sicurezza Critica (2h)

```go
// 1. Rimuovere api_key da /demo/config
// 2. Aggiungere flag cfg.Demo.Enabled per controllare l'endpoint
// 3. Proteggere /test/start con API key o limitarlo a localhost
```

### Step 2 — Rate Limiting (1h)

```bash
go get github.com/go-chi/httprate
```

```go
r.Use(httprate.LimitByIP(100, time.Minute))
r.With(httprate.LimitByIP(10, time.Minute)).Post("/v1/sessions", ...)
```

### Step 3 — CORS Configurabile (1h)

```yaml
# config.yaml
api:
  cors:
    allowed_origins: ["*"]  # default dev, override in prod
```

### Step 4 — Error Response Unificata (1h)

Usare `internal/api/response/` package già esistente (se c'è) o crearlo, e aggiornare tutti gli handler.

### Step 5 — GET /v1/sessions + Paginazione (3-4h)

Aggiungere handler e query SQL con `LIMIT/OFFSET`.

### Step 6 — Swagger UI (2h)

```go
import "github.com/swaggo/http-swagger"
r.Get("/docs/*", httpSwagger.Handler(httpSwagger.URL("/v1/openapi.yaml")))
r.Get("/v1/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "specs/contracts/api.yaml")
})
```
