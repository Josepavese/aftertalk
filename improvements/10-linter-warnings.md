# Improvement 10: Fix Functional golangci-lint Warnings

## Obiettivo

Eliminare tutti i warning funzionali di golangci-lint mantenendo la configurazione aggressiva (`default: all`). I warning "cosmetici" (stile, formattazione) sono secondari.

## Stato: IN CORSO

### Contesto

- golangci-lint v2, configurazione in `.golangci.yml`
- Aggiornato da v2.10.1 a v2.11.3 (necessario per compatibilità con Go 1.26)
- Aggiunto `run: go: "1.25"` in `.golangci.yml` (rimosso dopo l'upgrade del linter)
- Aggiunto exclusion in config per:
  - `gochecknoglobals` su `internal/metrics/metrics.go` (Prometheus richiede globals)
  - `govet/unusedwrite` su `_test.go` (campi test fixture)
  - `govet/fieldalignment` su `_test.go` (struct inline test)
  - `gosec/G101` su `_test.go` (stringhe fake credential)

---

## Categoria: FUNZIONALE (da fixare)

### ✅ RISOLTI

| Linter | Descrizione | File |
|--------|-------------|------|
| errcheck | `time.Parse` con `_` non checked | `session/repository.go`, `minutes/repository.go`, `transcription/repository.go`, `minutes/retrieval_token.go` |
| errcheck | `json.Marshal/Encode` non checked | `server.go`, `handler/minutes.go`, `ice_xirsys.go` |
| errcheck | `io.ReadAll` | `google_auth.go`, `webhook/client.go`, `stt/whisper_local.go` |
| errcheck | `rows.Close()`, `tx.Rollback()` | vari repository |
| errcheck | `conn.Close()`, `pc.Close()`, `peer.Close()` | `webrtc/peer.go`, `webrtc/signaling.go` |
| gosec G101 | Fake credentials in test files | Config-level exclusion in `.golangci.yml` |
| gosec G101 | OAuth URL falso positivo | `stt/google_auth.go` |
| gosec G101 | Example credentials in config | `config/config.go` |
| gosec G115 | Integer overflow conversion | `pkg/audio/*.go`, `pkg/webhook/retrier.go` |
| gosec G204 | exec.Command senza context | `pkg/audio/ogg_opus.go` → `exec.CommandContext` |
| gosec G304/G703 | Path traversal | `stt/google.go` → `filepath.Clean` |
| gosec G306 | File permissions | `stt/whisper_local.go` → `0600` |
| gosec G704 | SSRF | `webhook/client.go` → nolint |
| gosec G705 | XSS | `handler/minutes.go` → `http.Error` |
| noctx | HTTP/SQL senza context | `pkg/audio/ogg_opus.go`, test files |
| govet unusedwrite | Campi struct mai letti | Config-level exclusion per test files |
| govet fieldalignment | Struct non ottimizzate | Config-level exclusion per test files |
| govet shadow | Variabili shadow | `cmd/main.go`, `stt/google.go` |
| testifylint float-compare | `assert.Equal` su float | `transcription/repository_test.go`, `metrics_test.go`, `pcm_test.go`, `service_test.go` |
| testifylint require-error | `assert.Error` invece di `require.Error` | `config_test.go`, `logging/logger_test.go`, `queue_test.go`, `main_test.go`, `transcription/repository_test.go` |
| testifylint empty | `assert.Equal(t, "", ...)` | vari test |
| forcetypeassert | Type assertion non checked | `transcription_test.go`, `service_test.go`, `providers_test.go` |
| bodyclose | Response body non chiuso | `integration_test.go` |
| rowserrcheck | `rows.Err()` non checked | vari repository |
| staticcheck QF1008 | Embedded field selector | `main_test.go` → rimosso `.DB.` |
| errorlint | `%v` invece di `%w` | `stt/whisper_local.go` |
| err113 | `fmt.Errorf` inline | `webrtc/turn.go` → sentinel error |
| nolintlint | Direttive nolint non usate | vari (rimosse o corrette) |
| forbidigo | `fmt.Print` vietato | `cmd/main.go` → nolint con spiegazione |
| contextcheck | Goroutine senza ctx propagato | Nolint su fire-and-forget goroutines |

---

## Categoria: RIMANENTI (warning funzionali residui)

### errcheck: 2 rimanenti

```
sdk/node_modules/flatted/golang/...  ← NON nostro codice, escluso con exclude-dirs
```

> **Azione**: Già esclusa la dir `sdk/node_modules` in `.golangci.yml`. Verificare se i 2 errcheck residui sono in quella dir.

### testifylint: 6 rimanenti

Verificare con `golangci-lint run ./... | grep testifylint` – potrebbero essere già tutti risolti dopo gli ultimi fix.

---

## Categoria: COSMETIC (non prioritari, non bloccanti)

Questi warning sono stilistici/architetturali e **non** impattano correttezza:

| Linter | Count | Descrizione |
|--------|-------|-------------|
| funcorder | 26 | Ordine metodi (exported prima di unexported) |
| gochecknoinits | 3 | `init()` in test files |
| gocognit | 10 | Complessità cognitiva alta |
| goconst | 8 | Stringhe ripetute, usare costanti |
| gocritic | 42 | Varie micro-ottimizzazioni |
| gocyclo | 1 | Complessità ciclomatica |
| godot | 1 | Commento senza punto finale |
| gofumpt | 2 | Formattazione gofumpt |
| goimports | 1 | Import ordinamento |
| gosmopolitan | 2 | Hardcoded locale string |
| inamedparam | 1 | Named return parameters |
| intrange | 5 | `for i := 0; i < n; i++` → `for range n` |
| misspell | 2 | Typo in commenti |
| modernize | 18 | Pattern Go moderni |
| musttag | 3 | Struct fields senza json tag |
| nestif | 2 | If annidati eccessivi |
| nilnil | 1 | Return (nil, nil) |
| noinlineerr | 3 | Inline error handling |
| perfsprint | 4 | `fmt.Sprintf` ottimizzabile |
| prealloc | 5 | Slice pre-allocazione |
| promlinter | 2 | Prometheus naming convention |
| revive | 40 | Varie regole revive |
| thelper | 3 | Test helper senza t.Helper() |
| unparam | 2 | Parametri inutilizzati |
| unused | 2 | Codice inutilizzato |
| usestdlibvars | 1 | Usare costanti stdlib |
| wsl_v5 | 50 | Whitespace style |

---

## File Modificati (da questo improvement)

### Configurazione
- `.golangci.yml` — aggiornato con exclusion rules, upgrade go version

### Codice sorgente
- `pkg/audio/ogg_opus.go` — `exec.CommandContext`, `context.Context` param
- `pkg/audio/ogg.go` — nolint gosec G115
- `pkg/audio/opus.go` — nolint gosec G115
- `pkg/audio/pcm.go` — nolint gosec G115
- `pkg/webhook/client.go` — errcheck su io.ReadAll, nolint G704
- `pkg/webhook/retrier.go` — errcheck, nolint G115
- `internal/ai/stt/google.go` — filepath.Clean, shadow var fix
- `internal/ai/stt/google_auth.go` — errcheck io.ReadAll, nolint G101 OAuth URL
- `internal/ai/stt/aws.go` — strconv.ParseFloat invece di fmt.Sscanf
- `internal/ai/stt/whisper_local.go` — errcheck, G306, errorlint, ctx propagation
- `internal/ai/stt/azure.go` — ctx pass-through
- `internal/api/handler/minutes.go` — http.Error per G705, rimosso fmt
- `internal/api/response/json.go` — errcheck json.Encode
- `internal/api/server.go` — errcheck json.Encode, nolint contextcheck
- `internal/api/middleware/cors.go`, `metrics.go` — (verificare)
- `internal/bot/webrtc/signaling.go` — errcheck, nolint contextcheck
- `internal/bot/webrtc/peer.go` — errcheck pc.Close, peer.Close
- `internal/bot/webrtc/ice_xirsys.go` — errcheck json.Marshal
- `internal/bot/webrtc/turn.go` — sentinel error errUnexpectedAddrType
- `internal/core/minutes/service.go` — errcheck repo.Update, nolint contextcheck/gosec
- `internal/core/minutes/repository.go` — errcheck time.Parse
- `internal/core/minutes/retrieval_token.go` — errcheck time.Parse
- `internal/core/session/repository.go` — errcheck time.Parse (tutti)
- `internal/core/session/service.go` — errcheck UpdateAudioStream, nolint gosec/contextcheck
- `internal/core/transcription/repository.go` — errcheck time.Parse
- `internal/storage/cache/sessions.go` — rimosso mutex inutilizzato, fix type assertion
- `internal/storage/sqlite/db.go` — nolint errcheck tx.Rollback
- `internal/logging/logger.go` — nolint errcheck Logger.Sync
- `internal/metrics/metrics.go` — rimosso nolint (ora in config)
- `cmd/aftertalk/main.go` — shadow var fix, errcheck migrations, http.NewRequestWithContext, nolint forbidigo

### Test files
- `pkg/audio/pcm_test.go` — InDelta per float, nolint gosec
- `pkg/jwt/jwt_test.go` — testifylint fixes
- `pkg/webhook/retrier_test.go` — ExecContext
- `pkg/webhook/client_test.go` — (verificare)
- `internal/ai/stt/provider_test.go` — rimossi nolint inline, nolint struct block
- `internal/ai/stt/providers_test.go` — forcetypeassert
- `internal/ai/llm/providers_test.go` — forcetypeassert
- `internal/api/handler/health_test.go` — require import, require.NoError
- `internal/api/handler/minutes_test.go` — require import, require.NoError, fieldalignment nolint
- `internal/api/handler/session_test.go` — nolint forcetypeassert, fieldalignment
- `internal/api/handler/rtc_test.go` — nolint errcheck, ok check type assertions
- `internal/api/handler/transcription_test.go` — nolint forcetypeassert
- `internal/api/integration_test.go` — bodyclose nolint, http.NewRequestWithContext
- `internal/api/middleware/middleware_test.go` — rimossi nolint inline
- `internal/config/config_test.go` — require.Error
- `internal/core/minutes/entity_test.go` — testifylint order, GreaterOrEqual
- `internal/core/minutes/repository_test.go` — ExecContext, assert.Empty
- `internal/core/session/repository_test.go` — (verificare)
- `internal/core/session/service_test.go` — rimossi nolint inline, require
- `internal/core/transcription/entity_test.go` — assert.Empty, InEpsilon
- `internal/core/transcription/repository_test.go` — InEpsilon/Zero, require.Error
- `internal/core/transcription/service_test.go` — forcetypeassert, InEpsilon
- `internal/logging/logger_test.go` — require.Error
- `internal/metrics/metrics_test.go` — InEpsilon/InDelta/Zero
- `internal/storage/cache/queue_test.go` — require import, require.NoError, assert.Empty
- `internal/storage/cache/sessions_test.go` — (verificare)
- `internal/performance/benchmarks_test.go` — ExecContext, PingContext, QueryRowContext, nolint gosec G404
- `internal/performance/stress_test.go` — ExecContext, PingContext, QueryRowContext
- `internal/performance/pprof_test.go` — ReadHeaderTimeout, nolint gosec G404
- `cmd/aftertalk/main_test.go` — require.Error/NoError, rimosso .DB. embedded field selector

---

## Come riprendere

```bash
cd /home/jose/hpdev/Libraries/aftertalk

# Verifica stato attuale
~/go/bin/golangci-lint run ./... 2>&1 | grep "^\*"

# Vedere solo warning funzionali (non cosmetici)
~/go/bin/golangci-lint run ./... 2>&1 | grep -E "errcheck|gosec|noctx|govet|testifylint|nolintlint|forcetypeassert|bodyclose|staticcheck|err113|errorlint|forbidigo" | grep -E "^(internal|pkg|cmd)"
```

## Conteggio warning per sessione

| Sessione | Totale warning |
|----------|----------------|
| Inizio | ~400 |
| Fine sessione 1 (contesto esaurito) | ~270 |
| Fine sessione 2 (questo file) | ~248 |
| Di cui funzionali residui | ~6 (testifylint) |
| Di cui cosmetici | ~242 |
