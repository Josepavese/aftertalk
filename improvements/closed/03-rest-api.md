# Improvement: REST API — Modern, Secure, Organized

## Devil's Advocate Verdict

**The claim "well-organized, modern, simple and secure REST API" is PARTIALLY true.**

The basic structure is correct (chi router, v1 prefix, separate handlers, OpenAPI spec). But critical security and completeness components are missing for production readiness.

---

## Identified Gaps

### 1. CORS Wildcard — Insecure in Production

**Problem**: CORS is configured with `Access-Control-Allow-Origin: *` — accepts requests from any origin.

```go
// internal/api/middleware/middleware.go
// or server.go:101
cors.Handler(cors.Options{
    AllowedOrigins: []string{"*"},  // ← INSECURE
    AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders: []string{"*"},
})
```

**Impact**: In production, any website can make authenticated API requests if the user has a valid API key in the browser. Enables cross-origin CSRF attacks.

**Required Fix**:
```yaml
# config.yaml
api:
  cors:
    allowed_origins:
      - "https://app.mysite.com"
      - "http://localhost:3000"  # dev only
    allow_credentials: true
```

```go
// server.go — configurable CORS
cors.Handler(cors.Options{
    AllowedOrigins: cfg.API.CORS.AllowedOrigins,
    AllowCredentials: cfg.API.CORS.AllowCredentials,
})
```

---

### 2. No Rate Limiting

**Problem**: There is no protection against:
- Brute force on the API key
- DoS via many concurrent requests
- Abuse of POST /test/start (creates unlimited sessions)

Any form of throttling is completely absent.

**Required Fix**:
```go
// Add rate limiter middleware (golang.org/x/time/rate or go-chi/httprate)
import "github.com/go-chi/httprate"

r.Use(httprate.LimitByIP(100, 1*time.Minute))  // 100 req/min per IP

// Specific rate limit for expensive endpoints
r.With(httprate.LimitByIP(10, 1*time.Minute)).Post("/v1/sessions", ...)
```

---

### 3. `/demo/config` Exposes API Key — Critical Vulnerability

**Problem**: The `/demo/config` endpoint is public (no auth) and returns the server API key.

```go
// internal/api/server.go
r.Get("/demo/config", func(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]interface{}{
        "api_key":             cfg.API.Key,  // ← API KEY PUBLICLY EXPOSED
        "templates":           cfg.Templates,
        "default_template_id": defaultTemplateID,
    })
})
```

**Impact**: Anyone with access to the server URL can obtain the API key and make authenticated calls to the entire API. Acceptable for local demo, **not in production**.

**Required Fix**:
```go
// Separate the two concerns:

// 1. Public endpoint — only public metadata
r.Get("/v1/config", func(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]interface{}{
        "templates":           cfg.Templates,
        "default_template_id": defaultTemplateID,
        // NO api_key
    })
})

// 2. Local demo config — only if DEMO_MODE=true (env var)
if cfg.Demo.Enabled {
    r.Get("/demo/config", func(...) {
        // Can include api_key only in explicit demo mode
    })
}
```

---

### 4. `/test/start` Unprotected — Can Create Arbitrary Sessions

**Problem**: `POST /test/start` creates sessions without authentication. Anyone can create sessions on the server.

```go
// Public endpoint that creates sessions:
r.Post("/test/start", func(...) {
    // Creates session + participants + token
    // No auth required
})
```

**Required Fix**: This endpoint should either:
1. Require authentication (API key)
2. Be removed from production and available only with `--dev` flag
3. Have a limit of active sessions per IP

---

### 5. No Structured Input Validation

**Problem**: Input validation is ad-hoc and inconsistent across handlers.

**Examples of missing validation**:

```go
// handler/session.go — presumably:
// Does not verify that participant_count matches len(participants)
// Does not verify that roles are valid for the chosen template
// Does not verify that user_id is not empty
// Has no limits on string lengths (1MB user_id?)
```

```go
// handler/minutes.go:
// PUT /v1/minutes/{id} — accepts any JSON, no validation of sections format
```

**Required Fix**: Centralized validation with a library:
```go
// Option: github.com/go-playground/validator/v10
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

### 6. Inconsistent Error Format

**Problem**: Handlers return errors in different formats:

```go
// Some handlers:
http.Error(w, "Session ID required", http.StatusBadRequest)
// → plain text, not JSON

// Others:
json.NewEncoder(w).Encode(map[string]string{"error": msg})
// → JSON

// The OpenAPI spec declares:
// ErrorResponse: {error: string}
// But some endpoints return plain text
```

**Required Fix**: Centralize the error format:
```go
// internal/api/response/response.go
func Error(w http.ResponseWriter, status int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// All handlers use:
response.Error(w, http.StatusBadRequest, "Session ID required")
```

---

### 7. OpenAPI Spec Out of Sync with Code

**Problem**: `specs/contracts/api.yaml` (OpenAPI 3.0.3) exists but:
- It is not generated from code, it is hand-written
- It can go out-of-sync with real handlers
- There are no tests validating conformance
- Declared servers point to `api.aftertalk.io` but the server runs on port 8080

```yaml
# specs/contracts/api.yaml
servers:
  - url: https://api.aftertalk.io/v1    # ← non-existent domain
  - url: http://localhost:3000/v1        # ← wrong port (uses 8080)
```

**Required Fix**:
1. Automatic spec serving: `GET /v1/openapi.yaml` → serve the file
2. Embedded Swagger UI: `GET /docs` → interactive interface
3. Conformance tests: `go test` verifies that every route declared in the spec exists in the router
4. Automatic generation (long term): `swaggo/swag` or `ogen/ogen`

---

### 8. No Session Listing Endpoint

**Problem**: `GET /v1/sessions` (session list) does not exist. A client application cannot display session history.

**Missing API endpoints**:
| Endpoint | Use | Priority |
|---|---|---|
| `GET /v1/sessions` | List sessions (with filters, pagination) | High |
| `GET /v1/sessions/{id}/status` | Processing status polling | Medium |
| `DELETE /v1/sessions/{id}` | Delete session (GDPR) | High |
| `GET /v1/transcriptions/{id}` | Single transcription by ID | Medium |
| `DELETE /v1/minutes/{id}` | Delete minutes (GDPR) | Medium |

---

### 9. No Pagination

**Problem**: `GET /v1/transcriptions?session_id=...` returns all transcriptions without a limit. A long session can have hundreds of segments → potentially huge response.

**Required Fix**:
```
GET /v1/transcriptions?session_id=xxx&limit=50&offset=0
GET /v1/sessions?status=completed&limit=20&page=2&sort=created_at:desc
```

---

### 10. No Multi-Tenant Authentication Mechanism

**Problem**: The API has a single global API key. It is not possible to:
- Have multiple client applications with different keys
- Revoke a single key
- Track which client made which request

**Required Fix**: API key management:
```
POST /v1/api-keys        → Create new API key (requires master key)
GET  /v1/api-keys        → List API keys
DELETE /v1/api-keys/{id} → Revoke API key
```

---

## "Modern and Secure" Compliance Matrix

| Feature | Status | Notes |
|---|---|---|
| Versioning (`/v1/`) | ✅ Present | — |
| Bearer Token Auth | ✅ Present | Single global key |
| JWT for WebRTC | ✅ Present | — |
| CORS | ⚠️ Wildcard | Insecure in production |
| Rate Limiting | ❌ Absent | — |
| Input Validation | ⚠️ Partial | Ad-hoc, inconsistent |
| Consistent Error Format | ⚠️ Partial | Mix JSON/plain text |
| OpenAPI Spec | ⚠️ Present but stale | Out of sync, wrong servers |
| Swagger UI | ❌ Absent | — |
| Pagination | ❌ Absent | — |
| `/demo/config` secure | ❌ Exposes API key | Critical |
| `/test/start` protected | ❌ Public | Creates sessions without auth |
| GDPR endpoints | ❌ Absent | No DELETE session/minutes |
| Request logging | ✅ Present | Zap middleware |
| Request ID | ✅ Present | `X-Request-ID` |
| Global timeout | ✅ Present | 60s |
| Recovery (panic) | ✅ Present | — |

---

## Intervention Priority

| # | Gap | Impact | Effort | Priority |
|---|-----|--------|--------|----------|
| 3 | `/demo/config` exposes API key | Critical | Low | **Critical** |
| 4 | `/test/start` unprotected | High | Low | **High** |
| 2 | No rate limiting | High | Medium | **High** |
| 6 | Inconsistent errors (response package already exists) | Medium | Low | **High** |
| 1 | CORS wildcard | High | Low | **High** |
| 5 | Input validation | Medium | Medium | **Medium** |
| 8 | Missing endpoints (listing, delete) | Medium | Medium | **Medium** |
| 7 | Stale OpenAPI + no Swagger UI | Low | Medium | **Medium** |
| 9 | No pagination | Medium | Medium | **Medium** |
| 10 | Single API key | Low | High | **Low** |

---

## Implementation Steps

### Step 1 — Critical Security Fix (2h)

```go
// 1. Remove api_key from /demo/config
// 2. Add cfg.Demo.Enabled flag to control the endpoint
// 3. Protect /test/start with API key or restrict to localhost
```

### Step 2 — Rate Limiting (1h)

```bash
go get github.com/go-chi/httprate
```

```go
r.Use(httprate.LimitByIP(100, time.Minute))
r.With(httprate.LimitByIP(10, time.Minute)).Post("/v1/sessions", ...)
```

### Step 3 — Configurable CORS (1h)

```yaml
# config.yaml
api:
  cors:
    allowed_origins: ["*"]  # default dev, override in prod
```

### Step 4 — Unified Error Response (1h)

Use the existing `internal/api/response/` package (if present) or create it, and update all handlers.

### Step 5 — GET /v1/sessions + Pagination (3-4h)

Add handler and SQL query with `LIMIT/OFFSET`.

### Step 6 — Swagger UI (2h)

```go
import "github.com/swaggo/http-swagger"
r.Get("/docs/*", httpSwagger.Handler(httpSwagger.URL("/v1/openapi.yaml")))
r.Get("/v1/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "specs/contracts/api.yaml")
})
```
