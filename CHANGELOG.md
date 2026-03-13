# Changelog

All notable changes to Aftertalk are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)

---

## [Unreleased]

### Added
- Repository cleanup: unified documentation structure, standardized test layout, added LICENSE/CONTRIBUTING/CHANGELOG

---

## [0.8.0] - 2026-03-13

### Added
- **Secure minutes delivery** (`notify_pull` mode): webhook sends only a signed retrieval URL; recipient pulls data via single-use token; data purged after pull (HIPAA/GDPR-ready)
- `GET /v1/minutes/pull/{token}` endpoint — no API key needed, token is the credential
- `retrieval_tokens` table with atomic single-use consumption
- HMAC-SHA256 webhook signature (`X-Aftertalk-Signature` header)
- `WebhookConfig.Mode` field (`push` | `notify_pull`) with backward-compatible default

### Fixed
- `AudioManager.acquire()` resource leak: old stream now stopped before acquiring new one
- `anySignal()` listener leak: abort listeners cleaned up on combined signal fire

---

## [0.7.0] - 2026-03-10

### Added
- **SDK WebRTC resilience**: reconnection, ICE restart on failure, renegotiation after signaling reconnect, `tokenProvider` callback for JWT refresh
- 47 SDK unit tests (vitest, fake timers, MockWebSocket)

---

## [0.6.0] - 2026-03-08

### Added
- **TypeScript SDK** (`@aftertalk/sdk`): REST API clients, WebRTC realtime, minutes polling, error hierarchy
- Dual ESM+CJS build output

---

## [0.5.0] - 2026-03-05

### Added
- REST API security hardening: CORS, rate limiting, API key middleware
- Full CRUD for sessions, transcriptions, minutes (pagination, validation)
- WebRTC/ICE fixes: mDNS support, trickle ICE, thread-safe signaling

---

## [0.4.0] - 2026-03-01

### Added
- Self-contained deployment: embedded TURN server, ICE PAL (Google STUN → embedded TURN fallback)
- Modular installer (`scripts/install.sh`) with provider detection

---

## [0.1.0] - 2026-02-01

### Added
- Initial implementation: WebRTC audio capture, STT transcription, LLM minutes generation, webhook delivery
- Session lifecycle management with SQLite persistence
- PAL for STT (Google, AWS, Azure, Stub) and LLM (OpenAI, Anthropic, Azure, Stub) providers
- Template system for configurable minutes structure
