# Improvement: SDK JS/TS

## Devil's Advocate Verdict

**The claim "well-organized, structured, robust and modern JS/TS SDK" is FALSE.**

There is no JS/TS SDK. There is an HTML test file with inline JavaScript. The two things are not the same.

---

## Current State

```
cmd/test-ui/index.html   (509 lines, all inline)
├── HTML markup
├── CSS inline (<style>)
└── JavaScript inline (<script>)
    ├── WebRTC peer connection
    ├── WebSocket signaling
    ├── API calls (fetch)
    ├── UI state management
    └── Minutes rendering
```

**This is a demo prototype, not an SDK.**

Structural problems of the current implementation:

| Problem | Detail |
|---|---|
| All inline in 509 lines | Impossible to test, maintain, reuse |
| No TypeScript | No type safety, no autocompletion |
| No package | Not installable via npm/yarn |
| No bundler | Not integrable in React/Vue/Angular/Svelte |
| No tests | Zero unit or integration tests |
| No robust error handling | Sparse try/catch, no uniform strategy |
| No API documentation | No JSDoc, no typedoc |
| Hardcoded WebRTC config | STUN servers hardcoded in JS too |
| Manual minutes polling | `setInterval` at 5s without exponential backoff |
| No reconnect logic | WebSocket closes → no retry |
| Hardcoded UI strings | Not internationalizable |

---

## Proposed SDK Architecture

### Package Structure

```
sdk/
├── package.json
├── tsconfig.json
├── vitest.config.ts
└── src/
    ├── index.ts                  # Entry point, exports everything
    ├── client.ts                 # AfterthalkClient (main class)
    ├── types.ts                  # All TypeScript types
    ├── api/
    │   ├── sessions.ts           # SessionsAPI
    │   ├── transcriptions.ts     # TranscriptionsAPI
    │   ├── minutes.ts            # MinutesAPI
    │   └── config.ts             # ConfigAPI
    ├── webrtc/
    │   ├── connection.ts         # WebRTCConnection
    │   ├── signaling.ts          # SignalingClient (WebSocket)
    │   └── audio.ts              # AudioManager
    ├── realtime/
    │   └── minutes-poller.ts     # MinutesPoller (with backoff)
    └── errors.ts                 # AftertalkError hierarchy
```

---

### SDK API Design

#### 1. Main Client

```typescript
// Minimal usage
import { AfterthalkClient } from '@aftertalk/sdk';

const client = new AfterthalkClient({
  baseUrl: 'http://localhost:8080',
  apiKey: 'your-api-key',
});

// Create session
const session = await client.sessions.create({
  participantCount: 2,
  templateId: 'therapy',
  participants: [
    { userId: 'therapist-1', role: 'therapist' },
    { userId: 'patient-1',   role: 'patient' },
  ],
});

// Connect WebRTC
const connection = await client.webrtc.connect({
  sessionId: session.sessionId,
  token: session.participants[0].token,
});

// End session
await client.sessions.end(session.sessionId);

// Wait for minutes
const minutes = await client.minutes.waitForReady(session.sessionId, {
  timeout: 120_000,
  pollingInterval: 3_000,
});
```

#### 2. TypeScript Types (Aligned with Server)

```typescript
// src/types.ts

export interface Session {
  sessionId: string;
  status: 'active' | 'ended' | 'processing' | 'completed' | 'error';
  templateId?: string;
  participants: Participant[];
  createdAt: string;
  endedAt?: string;
}

export interface Participant {
  participantId: string;
  userId: string;
  role: string;
  token: string;
  connectedAt?: string;
  audioStreamId?: string;
}

export interface Minutes {
  id: string;
  sessionId: string;
  templateId: string;
  status: 'pending' | 'ready' | 'delivered' | 'error';
  sections: Record<string, unknown>;  // dynamic sections
  citations: Citation[];
  provider: string;
  version: number;
  generatedAt: string;
}

export interface Citation {
  text: string;
  role: string;
  timestampMs: number;
}

export interface Template {
  id: string;
  name: string;
  description: string;
  roles: RoleConfig[];
  sections: SectionConfig[];
}

export interface RoleConfig {
  key: string;
  label: string;
}

export interface SectionConfig {
  key: string;
  label: string;
  description: string;
  type: 'string_list' | 'content_items' | 'progress';
}

export interface Transcription {
  id: string;
  sessionId: string;
  role: string;
  text: string;
  status: 'pending' | 'processing' | 'ready' | 'error';
  confidence: number;
  startedAtMs: number;
  endedAtMs: number;
  createdAt: string;
}
```

#### 3. Sessions API

```typescript
// src/api/sessions.ts
export class SessionsAPI {
  constructor(private http: HttpClient) {}

  async create(request: CreateSessionRequest): Promise<CreateSessionResponse> { ... }
  async get(id: string): Promise<Session> { ... }
  async end(id: string): Promise<void> { ... }
  async list(filters?: SessionFilters): Promise<PaginatedResponse<Session>> { ... }
}
```

#### 4. WebRTC Connection

```typescript
// src/webrtc/connection.ts
export class WebRTCConnection extends EventEmitter {
  constructor(private config: WebRTCConfig) {}

  async connect(sessionId: string, token: string): Promise<void> { ... }
  async disconnect(): Promise<void> { ... }

  setMuted(muted: boolean): void { ... }

  // Events
  on('connected', (sessionId: string) => void): this;
  on('disconnected', (reason: string) => void): this;
  on('audio-started', () => void): this;
  on('ice-state-changed', (state: RTCIceConnectionState) => void): this;
  on('error', (error: Error) => void): this;
}
```

#### 5. Minutes Poller with Backoff

```typescript
// src/realtime/minutes-poller.ts
export class MinutesPoller {
  constructor(private api: MinutesAPI) {}

  async waitForReady(sessionId: string, options: PollerOptions): Promise<Minutes> {
    const { timeout = 120_000, minInterval = 2_000, maxInterval = 30_000 } = options;

    const deadline = Date.now() + timeout;
    let interval = minInterval;

    while (Date.now() < deadline) {
      const minutes = await this.api.getBySession(sessionId);

      if (minutes.status === 'ready' || minutes.status === 'delivered') {
        return minutes;
      }
      if (minutes.status === 'error') {
        throw new AftertalkError('minutes_generation_failed', minutes);
      }

      await sleep(interval);
      interval = Math.min(interval * 1.5, maxInterval);  // exponential backoff
    }

    throw new AftertalkError('minutes_polling_timeout', { timeout });
  }
}
```

#### 6. Error Hierarchy

```typescript
// src/errors.ts
export class AftertalkError extends Error {
  constructor(
    public readonly code: AftertalkErrorCode,
    public readonly details?: unknown,
    message?: string,
  ) {
    super(message ?? code);
    this.name = 'AftertalkError';
  }
}

export type AftertalkErrorCode =
  | 'network_error'
  | 'unauthorized'
  | 'not_found'
  | 'session_not_found'
  | 'minutes_generation_failed'
  | 'minutes_polling_timeout'
  | 'webrtc_connection_failed'
  | 'webrtc_ice_failed'
  | 'signaling_disconnected'
  | 'audio_permission_denied'
  | 'rate_limited';
```

#### 7. WebSocket Signaling with Reconnect

```typescript
// src/webrtc/signaling.ts
export class SignalingClient extends EventEmitter {
  private ws?: WebSocket;
  private reconnectAttempts = 0;
  private readonly maxReconnectAttempts = 5;

  async connect(url: string, token: string): Promise<void> {
    // Automatic reconnect with exponential backoff
    // onclose → if not intentional → reconnect with exponential delay
  }

  send(message: SignalingMessage): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    } else {
      this.messageQueue.push(message);  // queue for after reconnect
    }
  }
}
```

---

## Package Configuration

```json
// package.json
{
  "name": "@aftertalk/sdk",
  "version": "1.0.0",
  "description": "Official JavaScript/TypeScript SDK for Aftertalk API",
  "main": "./dist/cjs/index.js",
  "module": "./dist/esm/index.js",
  "types": "./dist/types/index.d.ts",
  "exports": {
    ".": {
      "import": "./dist/esm/index.js",
      "require": "./dist/cjs/index.js",
      "types": "./dist/types/index.d.ts"
    }
  },
  "scripts": {
    "build": "tsup src/index.ts --format cjs,esm --dts",
    "test": "vitest run",
    "test:watch": "vitest",
    "lint": "eslint src --ext .ts",
    "docs": "typedoc src/index.ts"
  },
  "peerDependencies": {
    "typescript": ">=5.0"
  },
  "devDependencies": {
    "typescript": "^5.4",
    "tsup": "^8.0",
    "vitest": "^1.0",
    "typedoc": "^0.25"
  }
}
```

---

## Test Strategy

```typescript
// src/api/__tests__/sessions.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { SessionsAPI } from '../sessions';

describe('SessionsAPI', () => {
  it('should create a session', async () => {
    const mockHttp = { post: vi.fn().mockResolvedValue({ sessionId: 'test-123', ... }) };
    const api = new SessionsAPI(mockHttp as any);

    const result = await api.create({ participantCount: 2, ... });

    expect(result.sessionId).toBe('test-123');
    expect(mockHttp.post).toHaveBeenCalledWith('/v1/sessions', expect.any(Object));
  });
});

// src/webrtc/__tests__/signaling.test.ts
// src/realtime/__tests__/minutes-poller.test.ts
```

---

## Framework Integration Examples

### React Hook

```typescript
// Example usage in React (not part of the core SDK, but documented)
import { useState, useEffect } from 'react';
import { AfterthalkClient } from '@aftertalk/sdk';

export function useAfterthalkSession(sessionId: string) {
  const [minutes, setMinutes] = useState<Minutes | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const client = new AfterthalkClient({ baseUrl: '/api', apiKey: '...' });

    client.minutes.waitForReady(sessionId, { timeout: 120_000 })
      .then(setMinutes)
      .finally(() => setLoading(false));
  }, [sessionId]);

  return { minutes, loading };
}
```

---

## Test UI Rewritten with SDK

Once the SDK is created, the test-ui should be rewritten using it:

```typescript
// cmd/test-ui/src/app.ts — test UI demonstrating the SDK
import { AfterthalkClient } from '@aftertalk/sdk';

const client = new AfterthalkClient({
  baseUrl: window.location.origin,
  apiKey: localStorage.getItem('aftertalk_api_key') ?? '',
});

// ↑ 3 lines instead of 509 lines of inline code
```

---

## Target Compatibility

| Environment | Support |
|---|---|
| Modern browsers (Chrome/Firefox/Safari/Edge) | ✅ |
| Node.js 18+ | ✅ (without WebRTC, HTTP API only) |
| Deno | ✅ (ESM) |
| React / Vue / Angular / Svelte | ✅ (via npm) |
| Next.js (SSR) | ✅ (server-side API only, WebRTC client-side only) |
| React Native | ⚠️ Partial (WebRTC requires native libs) |

---

## Intervention Priority

| # | Task | Effort | Priority |
|---|---|---|---|
| 1 | Define `types.ts` from OpenAPI spec | Low (2h) | **High** |
| 2 | Implement `HttpClient` with error handling | Medium (3h) | **High** |
| 3 | Implement `SessionsAPI`, `MinutesAPI`, `TranscriptionsAPI` | Medium (4h) | **High** |
| 4 | Implement `SignalingClient` with reconnect | High (6h) | **High** |
| 5 | Implement `WebRTCConnection` | High (8h) | **High** |
| 6 | Implement `MinutesPoller` with backoff | Low (2h) | **Medium** |
| 7 | Setup build (tsup + vitest) | Low (2h) | **High** |
| 8 | Unit tests for API layer | Medium (4h) | **High** |
| 9 | E2E integration tests (with real server) | High (8h) | **Medium** |
| 10 | Rewrite test-ui with SDK | Medium (4h) | **Medium** |
| 11 | Typedoc documentation + README | Medium (3h) | **Medium** |
| 12 | Publish npm package | Low (1h) | **Low** |

---

## Implementation Steps

### Step 1 — Scaffolding (2h)

```bash
mkdir sdk
cd sdk
npm init -y
npm install -D typescript tsup vitest @types/node
# Create tsconfig.json, vitest.config.ts
```

### Step 2 — Types from OpenAPI (2h)

Generate `types.ts` from `specs/contracts/api.yaml`:
```bash
npx openapi-typescript specs/contracts/api.yaml -o sdk/src/types.ts
```

### Step 3 — HttpClient + API Classes (4h)

Implement the 3 API classes with full error handling.

### Step 4 — WebRTC Layer (8h)

The most complex part: SignalingClient + WebRTCConnection with:
- ICE state management
- Reconnect logic
- Audio management
- Event emitter pattern

### Step 5 — Tests + Docs (4h)

Vitest unit tests + typedoc generation.
