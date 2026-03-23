# Aftertalk Integration Guide

## Overview

This guide describes the **canonical integration pattern** for Aftertalk: a TypeScript browser
frontend combined with a server-side PHP backend. The same principles apply to any backend language.

The central rule is simple:

> **The API key lives on the server. The browser receives only a short-lived JWT room token.**

---

## Table of Contents

1. [Architecture](#1-architecture)
2. [Security Model](#2-security-model)
3. [What Goes Where — Backend vs Frontend](#3-what-goes-where--backend-vs-frontend)
4. [Backend Layer — PHP SDK](#4-backend-layer--php-sdk)
5. [Frontend Layer — TypeScript SDK](#5-frontend-layer--typescript-sdk)
6. [Full Request Flow](#6-full-request-flow)
7. [Race Condition — Two Peers, One Room](#7-race-condition--two-peers-one-room)
8. [Ending a Session](#8-ending-a-session)
9. [Webhook Processing](#9-webhook-processing)
10. [Configuration Reference](#10-configuration-reference)

---

## 1. Architecture

```
┌────────────────────────────────────────────────────────────────────┐
│                         BROWSER (JS/TS)                            │
│                                                                    │
│  AftertalkClient (no API key)                                      │
│  ├── GET /v1/config          → load templates & profiles           │
│  ├── GET /v1/rtc-config      → load ICE servers                    │
│  └── WebSocket /signaling    → WebRTC audio stream (JWT auth)      │
│                                                                    │
│  fetch() → YOUR PHP backend  → Aftertalk API (with API key)        │
│  ├── POST /your-api/rooms/join  → join or create a room            │
│  ├── POST /your-api/sessions/end → end session                     │
│  └── GET  /your-api/minutes     → poll minutes                     │
└────────────────────────────────────────────────────────────────────┘
         │ JWT token only (no API key)           ▲ JWT token + session_id
         ▼                                       │
┌─────────────────────────────────────────────────────────────────────┐
│                      YOUR PHP BACKEND                               │
│                                                                     │
│  Holds: AFTERTALK_API_KEY (env var, never sent to browser)          │
│                                                                     │
│  AftertalkClient (PHP SDK, with API key)                            │
│  ├── POST /v1/rooms/join     → create/join session, get JWT token   │
│  ├── POST /v1/sessions/{id}/end → trigger STT+LLM pipeline          │
│  └── GET  /v1/sessions/{id}/minutes → read generated minutes        │
└─────────────────────────────────────────────────────────────────────┘
         │ API key in Authorization: Bearer header
         ▼
┌──────────────────────────────────────────────────────────────────────┐
│                     AFTERTALK SERVER                                 │
│                                                                      │
│  ├── REST API (protected by API key)                                 │
│  ├── WebSocket /signaling    (protected by JWT room token)           │
│  ├── STT pipeline (Whisper / Google / AWS / Azure)                   │
│  └── LLM pipeline (OpenAI / Anthropic / Ollama)                      │
│                                                                      │
│  → POST YOUR_WEBHOOK_URL (minutes delivered after session ends)      │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 2. Security Model

### What is the API key?

The Aftertalk API key (`Authorization: Bearer <key>`) grants full control over the server:
create sessions, end sessions, read minutes, list all data. **It must never leave your server.**

### What is the JWT room token?

When a participant joins a room, Aftertalk issues a short-lived JWT. This token:

- Encodes `session_id`, `participant_id`, `role`, and expiry
- Is **not forgeable** (signed with `JWT_SECRET`)
- Is valid only for `/signaling` (WebSocket) — not for REST calls
- Has a short TTL (default 1 hour, configurable via `JWT_EXPIRATION`)

The browser uses this token as a query parameter when connecting to the signaling WebSocket:

```
wss://your-aftertalk-server.com/signaling?token=<JWT>
```

### What is the session ID?

The `session_id` is a UUID that identifies the recording session. It is **not a secret**:

- It is safe to send to the browser
- It is used by the frontend to poll for minutes (via your backend proxy)
- Your backend associates it with your own domain objects (appointment, meeting, etc.)

### Security boundaries at a glance

| Data                  | Server | Your PHP Backend | Browser |
|-----------------------|--------|-----------------|---------|
| `AFTERTALK_API_KEY`   | ✓      | ✓ (env var)     | ✗ never |
| `JWT_SECRET`          | ✓      | ✗               | ✗ never |
| `WEBHOOK_SECRET`      | ✓      | ✓ (env var)     | ✗ never |
| `session_id`          | ✓      | ✓               | ✓ safe  |
| JWT room token        | issues | passes through  | ✓ safe  |
| ICE server URLs       | ✓      | —               | ✓ safe  |
| Minutes content       | ✓      | ✓               | ✓ safe  |

---

## 3. What Goes Where — Backend vs Frontend

### Your PHP backend is responsible for

- **Authenticating your users** (your own login system)
- **Deciding the participant role** (e.g. `user_id == appointment.doctor_id` → therapist)
- **Creating or joining the Aftertalk room** via `POST /v1/rooms/join` with the API key
- **Enforcing business rules**: who can create a session, which template to use, which STT/LLM profile
- **Ending the session** via `POST /v1/sessions/{id}/end` when the call is over
- **Receiving the webhook** and associating the minutes with your appointment/record
- **Verifying the HMAC signature** on every incoming webhook before processing

### The TypeScript frontend (browser) is responsible for

- **Rendering UI**: room UI, session controls, minutes display
- **WebRTC audio streaming**: capture microphone, stream to Aftertalk signaling via JWT
- **Polling session state**: ask your PHP backend when minutes are ready
- **Reading public config**: templates, ICE servers (directly from Aftertalk — no API key needed)

### What the frontend must NOT do

- Call Aftertalk REST endpoints with an API key
- Decide or declare the participant's role (the role must come from your auth)
- Create sessions directly (race condition risk + API key exposure)
- End sessions directly (must be a server-side, authenticated action)

---

## 4. Backend Layer — PHP SDK

### Installation

```bash
composer require aftertalk/aftertalk-php guzzlehttp/guzzle
```

### Client initialization

```php
use Aftertalk\AftertalkClient;

$aftertalk = new AftertalkClient(
    baseUrl:       $_ENV['AFTERTALK_URL'],      // e.g. https://aftertalk.yourserver.com
    apiKey:        $_ENV['AFTERTALK_API_KEY'],   // never expose this
    webhookSecret: $_ENV['AFTERTALK_WEBHOOK_SECRET'],
);
```

### Joining or creating a room

The `rooms->join()` call is **idempotent by room code**: the first participant creates the session,
the second joins the existing one. Both receive a personal JWT token.

```php
// POST /your-api/rooms/join
function handleRoomJoin(AftertalkClient $aftertalk, User $user, Appointment $appointment): array
{
    // Role is determined server-side from your domain logic — never from user input.
    $role = ($user->id === $appointment->doctorId) ? 'therapist' : 'patient';

    $result = $aftertalk->rooms->join(
        code:       $appointment->roomCode,   // your stable room identifier
        name:       $user->displayName,
        role:       $role,
        templateId: 'therapy',                // or 'consulting', or a custom template id
        sttProfile: 'cloud',                  // optional: 'local' or 'cloud'
        llmProfile: 'cloud',                  // optional: 'local' or 'cloud'
    );

    // Save the session_id in your DB so you can link it to the appointment later.
    // The session_id is NOT a secret — it is safe to send to the browser.
    $appointment->saveAftertalkSession($result['sessionId']);

    return [
        'session_id' => $result['sessionId'],
        'token'      => $result['token'],    // short-lived JWT — browser uses this for WebRTC
    ];
}
```

### Ending a session

```php
// POST /your-api/sessions/end
function handleSessionEnd(AftertalkClient $aftertalk, string $sessionId): void
{
    // This triggers the STT transcription + LLM minute generation pipeline.
    // Minutes will be delivered to your WEBHOOK_URL when ready.
    $aftertalk->sessions->end($sessionId);
}
```

### Reading minutes directly (polling alternative to webhook)

```php
// GET /your-api/minutes?session_id=<id>
function handleGetMinutes(AftertalkClient $aftertalk, string $sessionId): array
{
    return $aftertalk->minutes->get($sessionId);
}
```

---

## 5. Frontend Layer — TypeScript SDK

### Installation

```bash
npm install @aftertalk/sdk
```

### Client initialization — no API key

```typescript
import { AftertalkClient } from '@aftertalk/sdk';

// baseUrl points to the same origin as the page — no API key.
const sdk = new AftertalkClient({
  baseUrl: window.location.origin,
});
```

### Loading server config (public endpoint — no API key required)

```typescript
const cfg = await sdk.config.getConfig();
// cfg.templates       → Template[]  — available minute structures
// cfg.sttProfiles     → string[]    — available STT profiles (e.g. ['local', 'cloud'])
// cfg.llmProfiles     → string[]    — available LLM profiles
// cfg.defaultTemplateId → string
```

### Joining a room via your PHP backend

The browser calls **your PHP API**, not Aftertalk directly.

```typescript
// Call YOUR backend — it holds the API key and determines the role.
const res = await fetch('/api/rooms/join', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    code:        roomCode,
    name:        userName,
    template_id: selectedTemplateId,
    stt_profile: selectedSttProfile,
    llm_profile: selectedLlmProfile,
    // Role is NOT sent here — your backend derives it from the authenticated user.
  }),
});

const { session_id, token } = await res.json();
```

### Connecting WebRTC with the JWT token

```typescript
// AftertalkClient.connectWebRTC() internally calls GET /v1/rtc-config (public)
// to fetch ICE servers, then connects to /signaling with the JWT token.
const connection = await sdk.connectWebRTC({
  sessionId: session_id,
  token:     token,       // JWT from your PHP backend — no API key involved
});

connection.on('connected',      () => console.log('streaming audio'));
connection.on('disconnected',   (reason) => console.log('disconnected:', reason));
connection.on('audio-started',  () => console.log('microphone active'));
connection.on('error',          (err) => console.error(err));
```

### Ending the session from the frontend

The browser calls **your PHP backend**, which calls Aftertalk.

```typescript
await fetch('/api/sessions/end', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ session_id }),
});

connection.disconnect();
```

### Polling for minutes via your PHP backend

```typescript
// Poll your PHP backend (which proxies to Aftertalk with API key).
// The SDK also has a built-in poller for direct use if your backend exposes the endpoint.
async function pollMinutes(sessionId: string): Promise<Minutes> {
  for (let attempt = 0; attempt < 60; attempt++) {
    const res = await fetch(`/api/minutes?session_id=${sessionId}`);
    const data = await res.json();

    if (data.status === 'ready' || data.status === 'delivered') {
      return data;
    }
    await new Promise(r => setTimeout(r, 5_000)); // wait 5 s between polls
  }
  throw new Error('Minutes not ready after 5 minutes');
}
```

---

## 6. Full Request Flow

```
User opens room URL
        │
        ▼
Browser: GET /api/user-info          → Your PHP (verify session/JWT)
Browser: GET /v1/config              → Aftertalk (public — no API key)
Browser: populate template/role UI
        │
        ▼ user clicks "Join"
Browser: POST /api/rooms/join        → Your PHP
         Your PHP: determine role from auth
         Your PHP: POST /v1/rooms/join (API key) → Aftertalk
         Aftertalk: create/find session, issue JWT
         Your PHP: save session_id in DB ← link to appointment
         Your PHP: return { session_id, token } → Browser
        │
        ▼
Browser: sdk.connectWebRTC({ session_id, token })
         SDK: GET /v1/rtc-config (public) → Aftertalk → ICE servers
         SDK: WebSocket wss://…/signaling?token=<JWT>
         Aftertalk: validate JWT, open WebRTC peer connection
         Browser microphone → Aftertalk (audio streaming begins)
        │
        ▼ call ends — user clicks "End"
Browser: POST /api/sessions/end { session_id } → Your PHP
         Your PHP: POST /v1/sessions/{id}/end (API key) → Aftertalk
         Aftertalk: close WebRTC, run STT → transcript, run LLM → minutes
         Aftertalk: POST YOUR_WEBHOOK_URL { minutes, metadata }
         Your PHP webhook handler: verify HMAC, save minutes, notify user
```

---

## 7. Race Condition — Two Peers, One Room

**The problem**: the doctor and the patient both press "Join" within milliseconds of each other.
Both browsers call `POST /api/rooms/join`. Both requests hit your PHP backend simultaneously.
Without protection, both might attempt to create the session.

**The solution**: Aftertalk's `POST /v1/rooms/join` is **idempotent by room code**.
The room code is your key (e.g. `appointment_id` or a stable short string).

- The first request to arrive creates the session and issues the first JWT.
- The second request finds the existing session and issues the second JWT.
- Both return `{ session_id, token }` — same `session_id`, different `token`.

**Your PHP backend should also protect against double-create** at the DB level:

```php
// Atomic get-or-create using your own DB, keyed on appointment_id.
// This prevents duplicate Aftertalk sessions even under concurrent load.
DB::transaction(function () use ($appointment, $aftertalk, $user) {
    $existing = AftertalkSession::where('appointment_id', $appointment->id)->lockForUpdate()->first();

    if ($existing) {
        // Session already exists — just join it.
        return $aftertalk->rooms->join(code: $existing->room_code, ...);
    }

    // First to arrive — create.
    $result = $aftertalk->rooms->join(code: $appointment->id, ...);
    AftertalkSession::create([
        'appointment_id' => $appointment->id,
        'session_id'     => $result['sessionId'],
        'room_code'      => $appointment->id,
    ]);
    return $result;
});
```

---

## 8. Ending a Session

Two patterns are supported:

### Explicit end (recommended as primary path)

The frontend signals the PHP backend when the user ends the call. The PHP backend calls
`POST /v1/sessions/{id}/end`. Aftertalk starts the STT + LLM pipeline immediately.

This is the fastest path to minutes — the pipeline starts as soon as `end` is called.

### Auto-timeout (safety net)

Configure `SESSION_MAX_DURATION` (e.g. `2h`) in Aftertalk. If the session is never explicitly ended
(browser crash, network drop), Aftertalk will auto-close after the timeout and still run the
pipeline. Your backend does not need to handle this case explicitly — the webhook will arrive
regardless.

**Recommendation**: implement both. Explicit end for the normal flow, auto-timeout as a guarantee
that minutes are always generated even when things go wrong.

---

## 9. Webhook Processing

Aftertalk delivers the generated minutes to `WEBHOOK_URL` after the session pipeline completes.

### Modes

| Mode          | Description |
|---------------|-------------|
| `push`        | Minutes are in the webhook body. Simple, one HTTP call. |
| `notify_pull` | Webhook body contains a signed one-time URL. Your backend must fetch minutes from that URL. |

For medical or otherwise sensitive data, prefer `notify_pull`: zero medical content travels in
the webhook body, only a signed URL with a short TTL.

### Payload structure

```json
{
  "session_id": "d972ce12-...",
  "status":     "ready",
  "template_id": "therapy",
  "version":    1,
  "generated_at": "2026-03-19T17:14:49Z",
  "sections": {
    "themes":                    ["Stato attuale della paziente", "..."],
    "contents_reported":         [{"text": "...", "timestamp": 22179}],
    "professional_interventions":[{"text": "...", "timestamp": 45000}],
    "progress_issues":           {"progress": ["..."], "issues": ["..."]},
    "next_steps":                ["Pausa terapeutica di un mese"]
  },
  "citations": [
    {"text": "La paziente riferisce di stare molto bene", "role": "patient", "timestamp_ms": 22179}
  ]
}
```

### HMAC verification (required)

Every incoming webhook must be verified before processing. The `X-Aftertalk-Signature` header
contains `sha256=<HMAC-SHA256(body, WEBHOOK_SECRET)>`.

```php
function handleWebhook(AftertalkClient $aftertalk): void
{
    $rawBody  = file_get_contents('php://input');
    $signature = $_SERVER['HTTP_X_AFTERTALK_SIGNATURE'] ?? '';

    // Reject immediately if signature is missing or invalid.
    if (!$aftertalk->webhook->verify($rawBody, $signature)) {
        http_response_code(401);
        exit;
    }

    $payload   = json_decode($rawBody, true);
    $sessionId = $payload['session_id'];

    // Find the appointment linked to this session.
    $session     = AftertalkSession::where('session_id', $sessionId)->firstOrFail();
    $appointment = $session->appointment;

    // Persist minutes to your domain.
    $appointment->saveMinutes([
        'sections'  => $payload['sections'],
        'citations' => $payload['citations'],
        'version'   => $payload['version'],
    ]);

    // Notify the doctor.
    $appointment->doctor->notify(new MinutesReadyNotification($appointment));

    http_response_code(200);
    echo json_encode(['ok' => true]);
}
```

### notify_pull flow

```php
// In notify_pull mode, the webhook body contains a pull URL instead of the minutes.
$pullUrl = $payload['pull_url'];  // signed, one-shot, short TTL

// Fetch the actual minutes using the signed URL (no API key needed — the URL is the auth).
$minutes = json_decode(file_get_contents($pullUrl), true);
```

---

## 10. Configuration Reference

### Aftertalk server environment variables

| Variable                | Required | Description |
|-------------------------|----------|-------------|
| `API_KEY`               | yes      | Bearer token for all REST API calls |
| `JWT_SECRET`            | yes      | Signs participant room tokens |
| `JWT_EXPIRATION`        | no       | Room token TTL (default: `1h`) |
| `WEBHOOK_URL`           | yes      | Where to deliver minutes |
| `WEBHOOK_SECRET`        | yes      | HMAC key for `X-Aftertalk-Signature` |
| `WEBHOOK_MODE`          | no       | `push` (default) or `notify_pull` |
| `STT_PROVIDER`          | yes      | `whisper-local`, `google`, `aws`, `azure` |
| `LLM_PROVIDER`          | yes      | `openai`, `anthropic`, `ollama`, `azure` |
| `DATABASE_PATH`         | no       | SQLite path (default: `./aftertalk.db`) |
| `HTTP_PORT`             | no       | Server port (default: `8080`) |

### Your PHP backend environment variables

```dotenv
AFTERTALK_URL=https://aftertalk.yourserver.com
AFTERTALK_API_KEY=<secret — never commit, never log>
AFTERTALK_WEBHOOK_SECRET=<same value as server WEBHOOK_SECRET>
```

### Public endpoints (no API key — safe for frontend)

| Endpoint            | Description |
|---------------------|-------------|
| `GET /v1/config`    | Templates, STT/LLM profiles, defaults |
| `GET /v1/rtc-config`| ICE server list for WebRTC |

### Protected endpoints (API key required — backend only)

| Endpoint                           | Description |
|------------------------------------|-------------|
| `POST /v1/rooms/join`              | Create or join a room, issue JWT |
| `GET /v1/sessions`                 | List sessions |
| `GET /v1/sessions/{id}`            | Get session details |
| `POST /v1/sessions/{id}/end`       | End session, trigger pipeline |
| `GET /v1/sessions/{id}/transcriptions` | Raw transcript |
| `GET /v1/sessions/{id}/minutes`    | Generated minutes |
| `PUT /v1/sessions/{id}/minutes`    | Edit minutes |
| `GET /v1/sessions/{id}/minutes/versions` | Version history |
| `GET /v1/minutes/pull/{token}`     | One-shot pull URL (notify_pull mode, no API key — URL is auth) |

---

## Reference implementations

- **PHP middleware** (standalone, no Composer): [`cmd/test-ui/php/middleware.php`](../../cmd/test-ui/php/middleware.php)
- **TypeScript frontend**: [`cmd/test-ui/src/main.ts`](../../cmd/test-ui/src/main.ts)
- **PHP SDK**: [`sdk/php/`](../../sdk/php/)
- **TypeScript SDK**: [`sdk/ts/`](../../sdk/ts/)
