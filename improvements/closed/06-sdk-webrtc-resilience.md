# Improvement 06 — SDK WebRTC Resilience

## Devil's Advocate Verdict

**The claim "SDK is robust and resilient" is PARTIALLY FALSE for the WebRTC layer.**

The HTTP + MinutesPoller layer is genuinely robust. The WebRTC layer has 4 real bugs and 1 architectural gap that make the connection fragile in production.

---

## Documented Bugs and Gaps

### BUG 1 — Memory Leak: Listener Accumulation on WS Reconnect

**File**: `sdk/src/webrtc/signaling.ts`, line 100-101
**Severity**: High — memory leak + duplicated behavior

```typescript
// connect() called on every reconnect:
this.ws.addEventListener('message', this.onMessage);  // NOT once
this.ws.addEventListener('close', this.onClose);       // NOT once
```

After the 3rd reconnect there are **3 active `message` handlers** and **3 active `close` handlers**.
Every received message is processed 3 times. The backoff receives `onClose` 3 times,
starting 3 parallel timers.

**Fix**: remove listeners from the old WS before creating a new one, and register
`message`/`close` with explicit cleanup.

---

### BUG 2 — ICE `disconnected` Treated as Terminal

**File**: `sdk/src/webrtc/connection.ts`, line 145
**Severity**: High — false positive disconnections

```typescript
} else if (state === 'disconnected' || state === 'closed') {
  this.emit('disconnected', `ICE ${state}`);
}
```

The [W3C WebRTC spec](https://www.w3.org/TR/webrtc/#rtciceconnectionstate-enum) specifies:
> `disconnected`: "One or more components is running in a failed state **but there is
> a chance that the ICE agent will recover**."

The browser has an internal timer (typically 5s) after which it may return to `connected`
on its own (e.g. WiFi → 4G switch, momentary jitter). Emitting `disconnected`
immediately terminates the session for a transient condition.

**Fix**: wait for transition to `failed` before emitting `disconnected`.
Use a 5s grace timer on `disconnected`.

---

### BUG 3 — No ICE Restart on `failed`

**File**: `sdk/src/webrtc/connection.ts`, line 143-144
**Severity**: High — no automatic recovery from ICE failure

```typescript
} else if (state === 'failed') {
  this.emit('error', new AftertalkError('webrtc_ice_failed', ...));
  // ← stops here
}
```

When ICE enters `failed`, the correct recovery is:
1. `pc.restartIce()` — signals the browser to gather new candidates
2. Create a new offer with `{ iceRestart: true }`
3. Send the new offer via signaling
4. Wait for the new answer

Without this, any network interruption > ~30s leads to permanent audio loss,
requiring a manual reload.

**Fix**: implement `attemptICERestart()` with configurable max retries (default 3).

---

### BUG 4 — Signaling Reconnect Does Not Renegotiate the PeerConnection

**File**: `sdk/src/webrtc/connection.ts`, line 88-90
**Severity**: Medium — silent audio failure post-reconnect

```typescript
this.signaling.on('disconnected', (reason) => this.emit('disconnected', reason));
this.signaling.on('reconnecting', (attempt) => this.emit('signaling-reconnecting', attempt));
```

When the WS reconnects (after `reconnecting`), `SignalingClient` emits `connected`
but `WebRTCConnection` does not listen to that event. If during the WS disconnection
the PeerConnection lost ICE, no new offer is sent → audio remains silent without visible errors.

**Fix**: listen to `SignalingClient.on('connected')` in `WebRTCConnection` and check
ICE state. If it is `failed` or `disconnected`, trigger ICE restart.

---

### GAP 5 — Token Expiry on Reconnect Not Handled

**File**: `sdk/src/webrtc/signaling.ts`, line 80
**Severity**: Medium — silent 401 loop until max attempts

```typescript
const wsUrl = `${this.url}?token=${encodeURIComponent(this.token)}`;
```

The JWT token is fixed at construction and reused on every reconnect. With a short
TTL JWT (e.g. 1h), after expiry every attempt receives 401/403 and is interpreted
as a network error → backoff → new attempt → 401 again.
The user never sees an explicit `unauthorized` error.

**Fix**: accept an optional `tokenProvider: () => string | Promise<string>` callback
in the constructor. If present, it is invoked on every connection attempt.

---

## Fix Architecture

### SignalingClient — changes

```typescript
export interface SignalingClientOptions {
  url: string;
  token: string;
  maxReconnectAttempts?: number;
  // NEW: callback for token refresh
  tokenProvider?: () => string | Promise<string>;
  // NEW: jitter on backoff (0-1, default 0.3)
  backoffJitter?: number;
}
```

**Listener cleanup**:
```typescript
private attachListeners(ws: WebSocket): void {
  ws.addEventListener('message', this.onMessage);
  ws.addEventListener('close', this.onClose);
}

private detachListeners(ws: WebSocket): void {
  ws.removeEventListener('message', this.onMessage);
  ws.removeEventListener('close', this.onClose);
}

// In connect(), before creating the new WS:
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

**Backoff with jitter**:
```typescript
private backoffDelay(): number {
  const base = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30_000);
  const jitter = this.options.backoffJitter ?? 0.3;
  return base * (1 - jitter + Math.random() * jitter * 2);
}
```

---

### WebRTCConnection — changes

**Grace on `disconnected`**:
```typescript
private iceDisconnectedTimer?: ReturnType<typeof setTimeout>;

// In oniceconnectionstatechange:
if (state === 'disconnected') {
  // Wait 5s: might self-recover
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

**Renegotiation post-signaling-reconnect**:
```typescript
this.signaling.on('connected', async () => {
  const iceState = this.pc?.iceConnectionState;
  if (iceState === 'failed' || iceState === 'disconnected') {
    await this.attemptICERestart();
  }
});
```

---

## Tests to Add

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

## New Public Types

```typescript
// WebRTCConfig (extended)
export interface WebRTCConfig {
  signalingUrl?: string;
  iceServers?: ICEServer[];
  maxReconnectAttempts?: number;
  audioConstraints?: MediaTrackConstraints;
  // NEW:
  tokenProvider?: () => string | Promise<string>;
  backoffJitter?: number;
  iceDisconnectedGraceMs?: number;  // default 5000
  maxIceRestarts?: number;           // default 3
}

// New event in ConnectionEventMap:
'ice-restarting': [attempt: number];
```

---

## Impact and Compatibility

- **Breaking changes**: none — all new parameters are optional
- **Behavior change**: `disconnected` emitted with 5s delay on ICE `disconnected`
  (breaking for anyone using the browser timeout as a trigger) → document
- **New events**: `ice-restarting` is non-breaking (additive)

---

## Implementation Priority

| # | Task | Effort | Priority |
|---|---|---|---|
| 1 | Fix listener accumulation in SignalingClient | Low (30min) | **Critical** |
| 2 | ICE `disconnected` grace timer | Low (20min) | **High** |
| 3 | ICE restart on `failed` | Medium (1h) | **High** |
| 4 | Renegotiation post-signaling-reconnect | Low (30min) | **High** |
| 5 | Token provider callback | Low (30min) | **Medium** |
| 6 | Backoff jitter | Low (15min) | **Low** |
| 7 | Tests for all fixes | Medium (2h) | **High** |
