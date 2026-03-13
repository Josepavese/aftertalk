# 09 — Code Quality: Bugs Found During Wiki Reverse Engineering

> Bugs discovered by systematic function-by-function code reading during wiki creation (March 2026).
> Each item is verified from source — file and line references included.

---

## 1. `Server.Shutdown(nil)` — nil context

**File**: `internal/api/server.go`
**Severity**: Medium — on graceful shutdown, `http.Server.Shutdown(nil)` panics in Go stdlib (nil context is invalid).

**Current code**:
```go
s.httpServer.Shutdown(nil)
```

**Fix**:
```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
s.httpServer.Shutdown(ctx)
```

---

## 2. Hardcoded development machine path in `findTestUIPath()`

**File**: `internal/api/server.go`
**Severity**: High — the function contains a hardcoded absolute path to the developer's machine. This will fail silently on any other deployment.

**Current code** (approx):
```go
"/home/jose/hpdev/Libraries/aftertalk/cmd/test-ui"
```

**Fix**: Remove the hardcoded path. The heuristic should only use relative paths from the binary's working directory or `os.Executable()`.

---

## 3. JWT expiration ignored in `CreateSession`

**File**: `internal/core/session/service.go`
**Severity**: Medium — `JWTConfig.Expiration` is loaded from config and validated, but `CreateSession` hardcodes `2*time.Hour` instead of using `cfg.JWT.Expiration`.

**Current code**:
```go
ExpiresAt: time.Now().Add(2 * time.Hour),
```

**Fix**:
```go
ExpiresAt: time.Now().Add(s.cfg.JWT.Expiration),
```

---

## 4. Race condition in `generateMinutesForSession` — `time.Sleep` instead of sync

**File**: `internal/core/session/service.go`
**Severity**: High — after calling `processRemainingAudio()` (async), the code uses `time.Sleep(2*time.Second)` to wait before generating minutes. If STT is slow or under load, transcriptions may not be persisted yet when LLM generation starts.

**Current code**:
```go
s.processRemainingAudio(ctx, sessionID)
time.Sleep(2 * time.Second)
s.generateMinutesForSession(ctx, sessionID)
```

**Fix**: `processRemainingAudio` should return a done channel or `sync.WaitGroup`, and `generateMinutesForSession` should wait on it. Alternatively, run both steps sequentially in the same goroutine.

---

## 5. Wrong HTTP status code for validation error in `CreateSession` handler

**File**: `internal/api/handler/session.go`
**Severity**: Low — the handler returns HTTP 500 for client-caused validation errors (e.g. fewer than 2 participants). REST convention requires 400 for client errors.

**Current code**:
```go
if len(req.Participants) < 2 {
    http.Error(w, "...", http.StatusInternalServerError)
}
```

**Fix**: Use `http.StatusBadRequest` (400).

---

## 6. HTTP client without timeout in LLM providers

**File**: `internal/ai/llm/providers.go`
**Severity**: Medium — `OpenAIProvider` and `AnthropicProvider` initialize `&http.Client{}` with no timeout. A slow or unresponsive LLM API will hang the goroutine indefinitely, blocking session completion.

**Current code**:
```go
httpClient: &http.Client{},
```

**Fix**: Use the configured LLM timeout (or a sensible default like 120s):
```go
httpClient: &http.Client{Timeout: 120 * time.Second},
```

---

## 7. `StubLLMProvider` hardcodes therapy section keys

**File**: `internal/ai/llm/providers.go`
**Severity**: Medium — the stub returns a hardcoded JSON blob with `themes`, `contents_reported`, etc. (therapy template keys). When running tests with the `consulting` or a custom template, stub output will fail template validation.

**Fix**: The stub should return empty sections keyed from the passed `TemplateConfig`, not hardcoded therapy keys.

---

## 8. `webhook/client.go` uses `log.Printf` instead of `logging.Infof`

**File**: `pkg/webhook/client.go`
**Severity**: Low — the rest of the codebase uses the structured zap logger from `internal/logging`. The webhook client uses the stdlib `log` package, producing differently formatted log lines.

**Fix**: Inject a `*zap.Logger` into `webhook.Client` (or use the package-level logger from `internal/logging`).

---

## 9. SDK class name typo: `AfterthalkClient` (double `h`)

**Files**: `sdk/src/client.ts`, `sdk/src/types.ts`
**Severity**: Low (breaking change) — the exported class and its config type are named `AfterthalkClient` / `AfterthalkClientConfig` (note `thal` instead of `tal`). This is inconsistent with the project name "Aftertalk".

**Fix**: Rename to `AftertalkClient` / `AftertalkClientConfig`. This is a **breaking change** for any existing consumers — ship as a major version bump with a deprecation alias.

---

## Priority Order

| # | Bug | Priority |
|---|---|---|
| 4 | Race condition (Sleep instead of sync) | P0 |
| 2 | Hardcoded dev path | P1 |
| 6 | HTTP client no timeout | P1 |
| 1 | Shutdown(nil) | P1 |
| 3 | JWT expiration ignored | P2 |
| 7 | Stub LLM wrong sections | P2 |
| 5 | Wrong HTTP 500 for 400 error | P2 |
| 8 | log.Printf instead of zap | P3 |
| 9 | SDK typo (breaking change) | P3 |
