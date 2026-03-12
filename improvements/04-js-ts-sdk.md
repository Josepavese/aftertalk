# Improvement: SDK JS/TS

## Verdetto Avvocato del Diavolo

**L'asserzione "SDK JS/TS ben organizzato, strutturato, robusto e moderno" è FALSA.**

Non esiste uno SDK JS/TS. Esiste un file HTML di test con JavaScript inline. Le due cose non sono la stessa cosa.

---

## Stato Attuale

```
cmd/test-ui/index.html   (509 righe, tutto inline)
├── HTML markup
├── CSS inline (<style>)
└── JavaScript inline (<script>)
    ├── WebRTC peer connection
    ├── WebSocket signaling
    ├── API calls (fetch)
    ├── UI state management
    └── Minutes rendering
```

**Questo è un prototipo di demo, non uno SDK.**

Problemi strutturali dell'implementazione attuale:

| Problema | Dettaglio |
|---|---|
| Tutto inline in 509 righe | Impossibile da testare, manutenere, riusare |
| Nessun TypeScript | Nessun type safety, nessun autocompletamento |
| Nessun package | Non installabile via npm/yarn |
| Nessun bundler | Non integrabile in React/Vue/Angular/Svelte |
| Nessun test | Zero test unitari o integration |
| Nessun error handling robusto | try/catch sparse, nessuna strategia uniforme |
| Nessuna documentazione API | Nessun JSDoc, nessun typedoc |
| Hardcoded WebRTC config | STUN servers hardcoded anche nel JS |
| Polling manuale minutes | `setInterval` a 5s senza exponential backoff |
| Nessun reconnect logic | WebSocket si chiude → nessun retry |
| Italiana UI hardcoded | Non internazionalizzabile |

---

## Architettura SDK Proposta

### Struttura Package

```
sdk/
├── package.json
├── tsconfig.json
├── vitest.config.ts
└── src/
    ├── index.ts                  # Entry point, esporta tutto
    ├── client.ts                 # AfterthalkClient (classe principale)
    ├── types.ts                  # Tutti i tipi TypeScript
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
    │   └── minutes-poller.ts     # MinutesPoller (con backoff)
    └── errors.ts                 # AftertalkError hierarchy
```

---

### API Design dello SDK

#### 1. Client Principale

```typescript
// Uso minimale
import { AfterthalkClient } from '@aftertalk/sdk';

const client = new AfterthalkClient({
  baseUrl: 'http://localhost:8080',
  apiKey: 'your-api-key',
});

// Creare sessione
const session = await client.sessions.create({
  participantCount: 2,
  templateId: 'therapy',
  participants: [
    { userId: 'therapist-1', role: 'therapist' },
    { userId: 'patient-1',   role: 'patient' },
  ],
});

// Connettere WebRTC
const connection = await client.webrtc.connect({
  sessionId: session.sessionId,
  token: session.participants[0].token,
});

// Terminare sessione
await client.sessions.end(session.sessionId);

// Attendere minuta
const minutes = await client.minutes.waitForReady(session.sessionId, {
  timeout: 120_000,
  pollingInterval: 3_000,
});
```

#### 2. Tipi TypeScript (Allineati al Server)

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

#### 5. Minutes Poller con Backoff

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

#### 7. WebSocket Signaling con Reconnect

```typescript
// src/webrtc/signaling.ts
export class SignalingClient extends EventEmitter {
  private ws?: WebSocket;
  private reconnectAttempts = 0;
  private readonly maxReconnectAttempts = 5;

  async connect(url: string, token: string): Promise<void> {
    // Gestione automatica reconnect con backoff
    // onclose → if not intentional → reconnect con delay esponenziale
  }

  send(message: SignalingMessage): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    } else {
      this.messageQueue.push(message);  // queue per dopo il reconnect
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
// Esempio uso in React (non parte dell'SDK core, ma documentato)
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

## Test UI Riscritta con SDK

Una volta creato l'SDK, il test-ui dovrebbe essere riscritto usandolo:

```typescript
// cmd/test-ui/src/app.ts — test UI che dimostra l'SDK
import { AfterthalkClient } from '@aftertalk/sdk';

const client = new AfterthalkClient({
  baseUrl: window.location.origin,
  apiKey: localStorage.getItem('aftertalk_api_key') ?? '',
});

// ↑ 3 righe invece di 509 righe di codice inline
```

---

## Compatibilità Target

| Ambiente | Supporto |
|---|---|
| Browser moderni (Chrome/Firefox/Safari/Edge) | ✅ |
| Node.js 18+ | ✅ (senza WebRTC, solo API HTTP) |
| Deno | ✅ (ESM) |
| React / Vue / Angular / Svelte | ✅ (via npm) |
| Next.js (SSR) | ✅ (solo API lato server, WebRTC solo client) |
| React Native | ⚠️ Parziale (WebRTC richiede lib native) |

---

## Priorità di Intervento

| # | Task | Effort | Priorità |
|---|---|---|---|
| 1 | Definire `types.ts` da spec OpenAPI | Basso (2h) | **Alta** |
| 2 | Implementare `HttpClient` con error handling | Medio (3h) | **Alta** |
| 3 | Implementare `SessionsAPI`, `MinutesAPI`, `TranscriptionsAPI` | Medio (4h) | **Alta** |
| 4 | Implementare `SignalingClient` con reconnect | Alto (6h) | **Alta** |
| 5 | Implementare `WebRTCConnection` | Alto (8h) | **Alta** |
| 6 | Implementare `MinutesPoller` con backoff | Basso (2h) | **Media** |
| 7 | Setup build (tsup + vitest) | Basso (2h) | **Alta** |
| 8 | Test unitari per API layer | Medio (4h) | **Alta** |
| 9 | Test integrazione E2E (con server reale) | Alto (8h) | **Media** |
| 10 | Riscrivere test-ui con SDK | Medio (4h) | **Media** |
| 11 | Documentazione typedoc + README | Medio (3h) | **Media** |
| 12 | Publish npm package | Basso (1h) | **Bassa** |

---

## Passi di Implementazione

### Step 1 — Scaffolding (2h)

```bash
mkdir sdk
cd sdk
npm init -y
npm install -D typescript tsup vitest @types/node
# Creare tsconfig.json, vitest.config.ts
```

### Step 2 — Types da OpenAPI (2h)

Generare `types.ts` da `specs/contracts/api.yaml`:
```bash
npx openapi-typescript specs/contracts/api.yaml -o sdk/src/types.ts
```

### Step 3 — HttpClient + API Classes (4h)

Implementare le 3 API classes con full error handling.

### Step 4 — WebRTC Layer (8h)

La parte più complessa: SignalingClient + WebRTCConnection con:
- Gestione stato ICE
- Reconnect logic
- Audio management
- Event emitter pattern

### Step 5 — Test + Docs (4h)

Vitest unit tests + typedoc generation.
