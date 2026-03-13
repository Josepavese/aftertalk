# Improvement 06 — SDK WebRTC Resilience

## Verdetto Avvocato del Diavolo

**L'asserzione "SDK robusto e resiliente" è PARZIALMENTE FALSA per il layer WebRTC.**

Il layer HTTP + MinutesPoller sono effettivamente robusti. Il layer WebRTC ha 4 bug reali e 1 gap architetturale che rendono la connessione fragile in produzione.

---

## Bug e Gap Documentati

### BUG 1 — Memory Leak: Listener Accumulation su WS Reconnect

**File**: `sdk/src/webrtc/signaling.ts`, riga 100-101
**Gravità**: Alta — memory leak + behavior duplicato

```typescript
// connect() chiamato ad ogni reconnect:
this.ws.addEventListener('message', this.onMessage);  // NON once
this.ws.addEventListener('close', this.onClose);       // NON once
```

Al 3° reconnect esistono **3 handler `message`** e **3 handler `close`** attivi.
Ogni messaggio ricevuto viene processato 3 volte. Il backoff riceve `onClose` 3 volte,
avviando 3 timer paralleli.

**Fix**: rimuovere i listener dal vecchio WS prima di crearne uno nuovo, e registrare
`message`/`close` con cleanup esplicito.

---

### BUG 2 — ICE `disconnected` Trattato Come Terminale

**File**: `sdk/src/webrtc/connection.ts`, riga 145
**Gravità**: Alta — falsi positivi di disconnessione

```typescript
} else if (state === 'disconnected' || state === 'closed') {
  this.emit('disconnected', `ICE ${state}`);
}
```

La [spec WebRTC W3C](https://www.w3.org/TR/webrtc/#rtciceconnectionstate-enum) specifica:
> `disconnected`: "One or more components is running in a failed state **but there is
> a chance that the ICE agent will recover**."

Il browser ha un timer interno (tipicamente 5s) dopo il quale può tornare a `connected`
autonomamente (es. cambio WiFi → 4G, jitter momentaneo). Emettere `disconnected`
immediatamente interrompe la sessione per una condizione transiente.

**Fix**: attendere la transizione a `failed` prima di emettere `disconnected`.
Usare un timer di grazia di 5s su `disconnected`.

---

### BUG 3 — Nessun ICE Restart su `failed`

**File**: `sdk/src/webrtc/connection.ts`, riga 143-144
**Gravità**: Alta — nessun recovery automatico da ICE failure

```typescript
} else if (state === 'failed') {
  this.emit('error', new AftertalkError('webrtc_ice_failed', ...));
  // ← si ferma qui
}
```

Quando ICE entra in `failed`, la recovery corretta è:
1. `pc.restartIce()` — segnala al browser di raccogliere nuovi candidati
2. Creare un nuovo offer con `{ iceRestart: true }`
3. Inviare il nuovo offer via signaling
4. Attendere il nuovo answer

Senza questo, qualsiasi interruzione di rete > ~30s porta alla perdita permanente
dell'audio, richiedendo un reload manuale.

**Fix**: implementare `attemptICERestart()` con max retry configurabile (default 3).

---

### BUG 4 — Signaling Reconnect Non Rinegozia la PeerConnection

**File**: `sdk/src/webrtc/connection.ts`, riga 88-90
**Gravità**: Media — silent audio failure post-reconnect

```typescript
this.signaling.on('disconnected', (reason) => this.emit('disconnected', reason));
this.signaling.on('reconnecting', (attempt) => this.emit('signaling-reconnecting', attempt));
```

Quando il WS si riconnette (dopo `reconnecting`), il `SignalingClient` emette `connected`
ma `WebRTCConnection` non ascolta quel evento. Se durante la disconnessione WS la
PeerConnection ha perso ICE, non viene inviato nessun nuovo offer → l'audio rimane
muto senza errori visibili.

**Fix**: ascoltare `SignalingClient.on('connected')` in `WebRTCConnection` e verificare
lo stato ICE. Se è `failed` o `disconnected`, avviare ICE restart.

---

### GAP 5 — Token Expiry su Reconnect Non Gestita

**File**: `sdk/src/webrtc/signaling.ts`, riga 80
**Gravità**: Media — loop silenzioso di 401 fino a max tentativi

```typescript
const wsUrl = `${this.url}?token=${encodeURIComponent(this.token)}`;
```

Il token JWT viene fissato al costruttore e riusato ad ogni reconnect. Con un JWT
a TTL breve (es. 1h), dopo la scadenza ogni tentativo riceve 401/403 e viene
interpretato come errore di rete → backoff → nuovo tentativo → ancora 401.
L'utente non vede mai un errore `unauthorized` esplicito.

**Fix**: accettare un callback `tokenProvider: () => string | Promise<string>`
opzionale nel costruttore. Se presente, viene invocato ad ogni tentativo di connessione.

---

## Architettura del Fix

### SignalingClient — modifiche

```typescript
export interface SignalingClientOptions {
  url: string;
  token: string;
  maxReconnectAttempts?: number;
  // NUOVO: callback per token refresh
  tokenProvider?: () => string | Promise<string>;
  // NUOVO: jitter sul backoff (0-1, default 0.3)
  backoffJitter?: number;
}
```

**Cleanup listener**:
```typescript
private attachListeners(ws: WebSocket): void {
  ws.addEventListener('message', this.onMessage);
  ws.addEventListener('close', this.onClose);
}

private detachListeners(ws: WebSocket): void {
  ws.removeEventListener('message', this.onMessage);
  ws.removeEventListener('close', this.onClose);
}

// In connect(), prima di creare il nuovo WS:
if (this.ws) {
  this.detachListeners(this.ws);
}
```

**Token refresh**:
```typescript
private async resolveToken(): Promise<string> {
  if (this.options.tokenProvider) {
    return await this.options.tokenProvider();
  }
  return this.token;
}
```

**Backoff con jitter**:
```typescript
private backoffDelay(): number {
  const base = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30_000);
  const jitter = this.options.backoffJitter ?? 0.3;
  return base * (1 - jitter + Math.random() * jitter * 2);
}
```

---

### WebRTCConnection — modifiche

**Grazia su `disconnected`**:
```typescript
private iceDisconnectedTimer?: ReturnType<typeof setTimeout>;

// In oniceconnectionstatechange:
if (state === 'disconnected') {
  // Aspetta 5s: potrebbe riconnettersi da solo
  this.iceDisconnectedTimer = setTimeout(() => {
    if (this.pc?.iceConnectionState === 'disconnected') {
      this.attemptICERestart();
    }
  }, 5_000);
} else {
  clearTimeout(this.iceDisconnectedTimer);
}
```

**ICE Restart**:
```typescript
private iceRestartAttempts = 0;
private readonly maxIceRestarts = 3;

private async attemptICERestart(): Promise<void> {
  if (!this.pc || !this.signaling || this.iceRestartAttempts >= this.maxIceRestarts) {
    this.emit('error', new AftertalkError('webrtc_ice_failed'));
    return;
  }

  this.iceRestartAttempts++;
  try {
    this.pc.restartIce();
    const offer = await this.pc.createOffer({ iceRestart: true });
    await this.pc.setLocalDescription(offer);
    this.signaling.send({ type: 'offer', sdp: offer.sdp ?? '' });
  } catch (err) {
    this.emit('error', new AftertalkError('webrtc_ice_failed', { details: err }));
  }
}
```

**Rinegoziazione post-signaling-reconnect**:
```typescript
this.signaling.on('connected', async () => {
  const iceState = this.pc?.iceConnectionState;
  if (iceState === 'failed' || iceState === 'disconnected') {
    await this.attemptICERestart();
  }
});
```

---

## Test da Aggiungere

```typescript
// signaling.test.ts
it('does not accumulate listeners after multiple reconnects')
it('calls tokenProvider on each reconnect attempt')
it('applies jitter to backoff delay')
it('emits unauthorized error on 401 instead of retrying')

// connection.test.ts
it('does not emit disconnected on transient ICE disconnected')
it('emits disconnected after grace period if still disconnected')
it('attempts ICE restart on ice failed')
it('renegotiates after signaling reconnect if ICE was failed')
it('stops after maxIceRestarts attempts')
```

---

## Nuovi Tipi Pubblici

```typescript
// WebRTCConfig (esteso)
export interface WebRTCConfig {
  signalingUrl?: string;
  iceServers?: ICEServer[];
  maxReconnectAttempts?: number;
  audioConstraints?: MediaTrackConstraints;
  // NUOVI:
  tokenProvider?: () => string | Promise<string>;
  backoffJitter?: number;
  iceDisconnectedGraceMs?: number;  // default 5000
  maxIceRestarts?: number;           // default 3
}

// Nuovo evento in ConnectionEventMap:
'ice-restarting': [attempt: number];
```

---

## Impatto e Compatibilità

- **Breaking changes**: nessuno — tutti i nuovi parametri sono opzionali
- **Behavior change**: `disconnected` emesso con 5s di ritardo su ICE `disconnected`
  (breaking per chi usa il timeout del browser come trigger) → documentare
- **Nuovi eventi**: `ice-restarting` non breaking (additive)

---

## Priorità di Implementazione

| # | Task | Effort | Priorità |
|---|---|---|---|
| 1 | Fix listener accumulation in SignalingClient | Basso (30min) | **Critica** |
| 2 | ICE `disconnected` grace timer | Basso (20min) | **Alta** |
| 3 | ICE restart su `failed` | Medio (1h) | **Alta** |
| 4 | Rinegoziazione post-signaling-reconnect | Basso (30min) | **Alta** |
| 5 | Token provider callback | Basso (30min) | **Media** |
| 6 | Backoff jitter | Basso (15min) | **Bassa** |
| 7 | Test per tutti i fix | Medio (2h) | **Alta** |
