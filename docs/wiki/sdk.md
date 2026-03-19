# TypeScript SDK

The `@aftertalk/sdk` package is a TypeScript client for the Aftertalk REST API and WebRTC audio pipeline.

## Installation

```bash
npm install @aftertalk/sdk
```

Requires: TypeScript 5+, modern browser or Node.js 18+ with fetch.

## Quick Start

```typescript
import { AftertalkClient } from '@aftertalk/sdk';

const client = new AftertalkClient({
  baseUrl: 'http://localhost:8080',
  apiKey: 'your-api-key',
});

// 1. Create a session
const session = await client.sessions.create({
  participantCount: 2,
  templateId: 'therapy',
  participants: [
    { userId: 'dott-rossi', role: 'therapist' },
    { userId: 'paziente-1', role: 'patient' },
  ],
});

// 2. Connect therapist via WebRTC
const token = session.participants.find(p => p.userId === 'dott-rossi')!.token;
const conn = await client.connectWebRTC({
  sessionId: session.sessionId,
  token,
});

// 3. End the session
await client.sessions.end(session.sessionId);

// 4. Wait for minutes to be ready (polls with exponential backoff)
const minutes = await client.waitForMinutes(session.sessionId);
console.log(minutes.sections);
```

## Client Configuration

```typescript
interface AftertalkClientConfig {
  baseUrl: string;      // Required: server URL e.g. "http://localhost:8080"
  apiKey?: string;      // Required for all /v1/* endpoints
  timeout?: number;     // Request timeout ms (default: 30000)
  fetch?: typeof fetch; // Custom fetch (useful for Node.js / testing)
}
```

## REST API Clients

The client exposes five sub-clients mirroring the REST API:

### `client.sessions`

```typescript
// Create a session
const session = await client.sessions.create({ participantCount, templateId?, participants });

// List sessions (paginated)
const list = await client.sessions.list({ status?, limit?, offset? });

// Get a session
const session = await client.sessions.get(sessionId);

// Get only status
const { status } = await client.sessions.getStatus(sessionId);

// End a session (idempotent)
await client.sessions.end(sessionId);

// Delete a session (must be ended first)
await client.sessions.delete(sessionId);
```

### `client.transcriptions`

```typescript
// List transcriptions for a session
const txns = await client.transcriptions.listBySession(sessionId, { limit?, offset? });

// Get a single transcription
const txn = await client.transcriptions.get(transcriptionId);
```

### `client.minutes`

```typescript
// Get minutes for a session
const minutes = await client.minutes.getBySession(sessionId);

// Get by ID
const minutes = await client.minutes.get(minutesId);

// Update minutes (saves previous version to history)
// userId is passed as X-User-Id header to track the editor
await client.minutes.update(minutesId, { sections }, userId?);

// Delete minutes
await client.minutes.delete(minutesId);

// Get edit history
const versions = await client.minutes.getVersions(minutesId);
```

### `client.config`

```typescript
// Get available templates, default template ID, and provider profiles
const { templates, defaultTemplateId, sttProfiles, llmProfiles } = await client.config.getConfig();

// Get ICE servers for WebRTC
const { iceServers } = await client.config.getRTCConfig();
```

### `client.rooms`

```typescript
// Join or create a room session by code
const { sessionId, token } = await client.rooms.join({
  code:       'stanza-abc',
  name:       'Dott. Rossi',
  role:       'terapeuta',
  templateId: 'therapy',
  sttProfile: 'cloud',   // optional
  llmProfile: 'cloud',   // optional
});
```

The room join endpoint creates the session on the first call; subsequent participants with the same code receive their own token for the same session. Role is exclusive — two participants cannot share the same role in the same room.

## WebRTC

### High-level: `connectWebRTC()`

Fetches ICE configuration automatically from the server, then connects:

```typescript
const conn = await client.connectWebRTC({
  sessionId: 'uuid',
  token: 'eyJ...',
  webrtcConfig?: {
    signalingUrl?: string,         // default: baseUrl → ws://.../signaling
    iceServers?: ICEServer[],      // default: fetched from /v1/rtc-config
    maxReconnectAttempts?: number, // signaling WS reconnects (default: 5)
    audioConstraints?: MediaTrackConstraints,
    tokenProvider?: () => string | Promise<string>,  // fresh token on reconnect
    backoffJitter?: number,        // jitter for reconnect backoff (default: 0.3)
    iceDisconnectedGraceMs?: number, // wait before ICE restart (default: 5000)
    maxIceRestarts?: number,       // max ICE restarts before error (default: 3)
  }
});
```

### Events

```typescript
conn.on('connected', (sessionId) => { /* ICE connected */ });
conn.on('disconnected', (reason) => { /* connection closed */ });
conn.on('audio-started', () => { /* microphone acquired */ });
conn.on('ice-state-changed', (state: RTCIceConnectionState) => { });
conn.on('ice-restarting', (attempt) => { /* ICE restart in progress */ });
conn.on('signaling-reconnecting', (attempt) => { });
conn.on('error', (err: AftertalkError) => { });
```

### ICE Resilience

The SDK handles WebRTC reconnection automatically:

1. **Disconnected**: waits `iceDisconnectedGraceMs` (5s) — browser may self-recover
2. **Failed / still-disconnected after grace**: triggers ICE restart via `createOffer({iceRestart:true})`
3. **Signaling reconnect**: if ICE was failed/disconnected, triggers ICE restart
4. After `maxIceRestarts` (default: 3) consecutive failures, emits `error`

### Mute / Disconnect

```typescript
conn.setMuted(true);   // silence mic without disconnecting
conn.setMuted(false);
await conn.disconnect();
```

## Minutes Polling

### Wait once

```typescript
const minutes = await client.waitForMinutes(sessionId, {
  timeout?: number,       // max wait ms (default: 120_000)
  minInterval?: number,   // initial poll interval ms (default: 2_000)
  maxInterval?: number,   // max poll interval ms (default: 30_000)
  backoffFactor?: number, // multiplier per poll (default: 1.5)
});
```

### Watch continuously

```typescript
await client.watchMinutes(sessionId, (minutes) => {
  console.log('New version:', minutes.version);
}, options);
```

## Error Handling

All SDK errors are instances of `AftertalkError`:

```typescript
import { AftertalkError } from '@aftertalk/sdk';

try {
  await client.sessions.get('non-existent-id');
} catch (err) {
  if (err instanceof AftertalkError) {
    console.log(err.code);    // e.g. "session_not_found", "webrtc_ice_failed"
    console.log(err.message);
    console.log(err.details);
  }
}
```

## Per-session STT/LLM provider profiles

When the server is configured with named STT and/or LLM profiles (e.g. `local` vs `cloud`),
you can select them at session-creation time:

```typescript
// Fetch available profiles from the server
const { sttProfiles, sttDefaultProfile, llmProfiles, llmDefaultProfile } =
  await client.config.getConfig();

// Create a session with explicit provider profiles
const session = await client.sessions.create({
  participantCount: 2,
  templateId: 'therapy',
  participants: [
    { userId: 'dott-rossi', role: 'therapist' },
    { userId: 'paziente-1', role: 'patient' },
  ],
  sttProfile: 'cloud',   // e.g. Groq whisper-large-v3
  llmProfile: 'cloud',   // e.g. OpenRouter minimax-m2.7
});
```

Profile names are defined server-side in the `stt.profiles` / `llm.profiles` configuration
sections. When omitted, the server falls back to its `stt.default_profile` /
`llm.default_profile`. The profile used is persisted on the session and visible in logs and
transcription records.

---

## Other SDKs

| SDK | Target | Package |
|-----|--------|---------|
| `@aftertalk/sdk` (this page) | Frontend / Node.js | npm |
| `aftertalk/aftertalk-php` | Backend PHP (Laravel, Symfony, …) | Composer / Packagist |
| `aftertalk/aftertalk-go` | Backend Go server-side clients | Go module |

See [sdk-php.md](sdk-php.md) for the PHP SDK documentation.
