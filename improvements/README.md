# Improvements — Critical Analysis

> Devil's advocate verdict on the 4 declared features.

---

## Executive Summary

| Declared Feature | Verdict | Real Status |
|---|---|---|
| Simple installer, SSOT, PAL | ✅ **Implemented** | Modular, `--dump-defaults` SSOT, systemd/launchd, `local-ai/cloud/offline` modes |
| Fullstack self-contained | ✅ **Implemented** | Embedded TURN (Pion), Google/AWS/Azure STT, configurable ICE, webhook retry |
| Modern and secure REST API | ✅ **Implemented** | Rate limiting, configurable CORS, secure `/demo/config`, new CRUD endpoints, input validation, pagination |
| Robust and modern JS/TS SDK | ✅ **Implemented** | `@aftertalk/sdk` complete TypeScript package with WebRTC, poller, 47 tests |

---

## All improvements completed

---

## Documents

### Completed (`closed/`)

- **[closed/01-installer.md](closed/01-installer.md)** — SSOT & PAL gaps in installer ✅
  - `ChunkSizeMs` and `TranscriptionQueueSize` in config
  - `--dump-defaults` flag (SSOT for config defaults)
  - Modular installer (`providers/_go.sh`, `_python.sh`, `_whisper.sh`, `_ollama.sh`)
  - Separate steps (`steps/_binary.sh`, `_config.sh`, `_cli.sh`)
  - `--mode=local-ai|cloud|offline` flag
  - Systemd/launchd integration (`aftertalk service install/uninstall`)

- **[closed/02-fullstack-selfcontained.md](closed/02-fullstack-selfcontained.md)** — Real self-containment ✅
  - Embedded TURN server (Pion/turn) with UDP+TCP
  - Configurable ICE provider (static/embedded/twilio/xirsys/metered)
  - Google/AWS/Azure STT implemented
  - Persistent webhook retry queue

- **[closed/03-rest-api.md](closed/03-rest-api.md)** — REST API security and completeness ✅
  - Secure `/demo/config` (API key only with `demo.enabled=true`)
  - `/test/start` protected by API key
  - Rate limiting wired (`cfg.API.RateLimit`)
  - Configurable CORS (`cfg.API.CORS`)
  - `GET /v1/sessions` with pagination (`?status=&limit=&offset=`)
  - `GET /v1/sessions/{id}/status`
  - `DELETE /v1/sessions/{id}`
  - `DELETE /v1/minutes/{id}`
  - Pagination on `GET /v1/transcriptions`
  - Input validation on `CreateSessionRequest` (user_id, role, length, count match)
  - `GET /v1/openapi.yaml`
  - `/v1/config` public endpoint (templates without API key)

- **[closed/04-js-ts-sdk.md](closed/04-js-ts-sdk.md)** — TypeScript SDK `@aftertalk/sdk` ✅
  - `types.ts`: complete types aligned to API (Session, Minutes, Transcription, Template, RTC)
  - `errors.ts`: `AftertalkError` with typed codes + HTTP status mapping
  - `http.ts`: `HttpClient` with timeout, API key header, error handling
  - `api/`: `SessionsAPI`, `MinutesAPI`, `TranscriptionsAPI`, `ConfigAPI`
  - `webrtc/`: `SignalingClient` (reconnect + backoff + message queue), `WebRTCConnection`, `AudioManager`
  - `realtime/`: `MinutesPoller` with `waitForReady()` + `watch()` + exponential backoff
  - `client.ts`: `AfterthalkClient` with `connectWebRTC()` and `waitForMinutes()` high-level
  - 27 unit tests (vitest), all passing
  - `cmd/test-ui/src/main.ts`: demo UI rewritten in TypeScript with SDK
  - Build: tsup (CJS + ESM + types), peer dep only TypeScript ≥5.0

- **[closed/05-webrtc-turn-stun.md](closed/05-webrtc-turn-stun.md)** — WebRTC/ICE fixes ✅

- **[closed/06-sdk-webrtc-resilience.md](closed/06-sdk-webrtc-resilience.md)** — SDK WebRTC resilience ✅
  - Fix memory leak: listener accumulation on WS reconnect (`attachListeners`/`detachListeners`)
  - ICE `disconnected` grace period (5s) before reacting
  - Automatic ICE restart on `failed` (`pc.restartIce()` + re-offer `{iceRestart:true}`)
  - Automatic renegotiation when signaling reconnects with ICE failed
  - `tokenProvider` callback for JWT refresh on reconnect
  - Configurable backoff jitter (`backoffJitter`) for thundering herd
  - WS close code 4001/4003 → immediate `unauthorized` error without retry
  - ICE restart counter reset only on `connected`/`completed`, not on answer

- **[07-secure-minutes-delivery.md](07-secure-minutes-delivery.md)** — Secure minutes delivery: notify_pull pattern ✅
  - `WebhookConfig.Mode`: `"push"` (legacy) | `"notify_pull"` (production/HIPAA/GDPR)
  - `retrieval_tokens` table: single-use, time-limited token scoped to a single minutes record
  - `GET /v1/minutes/pull/{token}`: pull endpoint authenticated by token, outside API key middleware
  - `webhook.NotificationPayload`: webhook body with retrieval URL only (zero sensitive data)
  - HMAC-SHA256 signature (`X-Aftertalk-Signature`) on notification webhooks
	- `PurgeMinutes`: deletes minutes + transcriptions after successful pull (`delete_on_pull=true`)
	- Retrier: `EnqueueNotification` + `payload_type` column for correct dispatch

- **[closed/30-runtime-build-identity.md](closed/30-runtime-build-identity.md)** — Runtime build identity and deploy verification ✅
  - `internal/version.BuildInfo` with semver, commit, tag, build time, and build source
  - `/v1/health` includes runtime build metadata; `/v1/version` exposes dedicated build identity
  - `aftertalk --version` and `aftertalk-installer --version`
  - Release workflow injects commit/tag/build time/source into server and installer binaries
  - Installer verification can fail on expected tag/commit mismatch

### Open

- **[09-code-quality-bugs.md](09-code-quality-bugs.md)** — 9 bugs found during wiki reverse engineering (race condition, nil context, hardcoded dev path, JWT expiry ignored, wrong HTTP status codes, HTTP client no timeout, stub LLM wrong keys, log stdlib, SDK typo)

- **[10-linter-warnings.md](10-linter-warnings.md)** — fix warning funzionali golangci-lint (errcheck, gosec, noctx, errorlint, forcetypeassert, rowserrcheck, staticcheck); configurazione aggressiva `default: all` mantenuta

- **[11-webhook-metadata.md](11-webhook-metadata.md)** — includere `session_metadata` e sommario partecipanti in `MinutesPayload` e `NotificationPayload`; prerequisito per integrazione MondoPsicologi

- **[12-session-auto-timeout.md](12-session-auto-timeout.md)** — collegare `SessionConfig.MaxDuration` (già in config, non ancora letto) a un background reaper nel service; SSOT già rispettato, manca solo il wiring

- **[13-php-sdk.md](13-php-sdk.md)** — PHP SDK `aftertalk/aftertalk-php` per integratori backend; affianca `@aftertalk/sdk` JS/TS; copre creazione sessioni, token, fine sessione, verifica webhook HMAC

- **[14-integration-guide-mondopsicologi.md](14-integration-guide-mondopsicologi.md)** — guida all'integrazione per piattaforme telemedicina con backend PHP e WebRTC custom; pattern: chi crea la sessione, race condition, assegnazione ruoli, metadata, webhook; dipende da #11 e #13

- **[29-ci-release-hardening-followups.md](29-ci-release-hardening-followups.md)** — incidenti emersi nel ciclo CI/release pubblico: root cause, workaround applicati e backlog hardening (auth header contract, metrics contract, coverage script, Node24 actions)
