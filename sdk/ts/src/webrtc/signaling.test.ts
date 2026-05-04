import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AftertalkError } from '../errors.js';
import { SignalingClient } from './signaling.js';

// ─── WebSocket mock ───────────────────────────────────────────────────────────

type WSEventHandler = (event: Event | MessageEvent | CloseEvent) => void;

class MockWebSocket {
  // Static constants matching browser WebSocket API
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  static instances: MockWebSocket[] = [];
  readyState = MockWebSocket.CONNECTING;

  private handlers: Map<string, Set<WSEventHandler>> = new Map();

  constructor(public url: string) {
    MockWebSocket.instances.push(this);
  }

  addEventListener(event: string, handler: WSEventHandler) {
    if (!this.handlers.has(event)) this.handlers.set(event, new Set());
    this.handlers.get(event)!.add(handler);
  }

  removeEventListener(event: string, handler: WSEventHandler) {
    this.handlers.get(event)?.delete(handler);
  }

  send(_data: string) {}
  close() { this.readyState = MockWebSocket.CLOSED; }

  triggerOpen() {
    this.readyState = MockWebSocket.OPEN;
    this._dispatch('open', new Event('open'));
  }

  triggerClose(code = 1006) {
    this.readyState = MockWebSocket.CLOSED;
    this._dispatch('close', { code, reason: '', wasClean: false } as CloseEvent);
  }

  triggerMessage(data: unknown) {
    this._dispatch('message', { data: JSON.stringify(data) } as MessageEvent);
  }

  listenerCount(event: string): number {
    return this.handlers.get(event)?.size ?? 0;
  }

  _dispatch(event: string, e: Event | MessageEvent | CloseEvent) {
    for (const h of this.handlers.get(event) ?? []) h(e);
  }
}

// ─── Setup ────────────────────────────────────────────────────────────────────

beforeEach(() => {
  MockWebSocket.instances = [];
  vi.stubGlobal('WebSocket', MockWebSocket);
  vi.useFakeTimers();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

function latestWS(): MockWebSocket {
  return MockWebSocket.instances.at(-1)!;
}

/**
 * Starts connect(), flushes resolveToken() microtask so WS is created,
 * then fires the open event and awaits the connect() promise.
 */
async function connectAndOpen(client: SignalingClient): Promise<void> {
  const prevCount = MockWebSocket.instances.length;
  const promise = client.connect();
  // Flush microtasks until the new WS is created. An async tokenProvider may
  // require more ticks than a sync one, so we loop until instances grows.
  for (let i = 0; i < 20; i++) {
    await Promise.resolve();
    if (MockWebSocket.instances.length > prevCount) break;
  }
  latestWS().triggerOpen();
  await promise;
}

// ─── Tests ────────────────────────────────────────────────────────────────────

describe('SignalingClient — listener cleanup', () => {
  it('does not accumulate message/close listeners after reconnect', async () => {
    const client = new SignalingClient({ url: 'ws://localhost', token: 'tok' });
    await connectAndOpen(client);

    const ws1 = latestWS();
    expect(ws1.listenerCount('message')).toBe(1);
    expect(ws1.listenerCount('close')).toBe(1);

    // Simulate a second connect() call (as reconnect does): detachListeners should
    // remove ws1's handlers before attaching to ws2.
    ws1.triggerClose();

    // Advance past reconnect delay (first attempt: ~1000ms with jitter)
    await vi.advanceTimersByTimeAsync(1500);
    // Flush resolveToken() microtask inside the reconnect callback
    await Promise.resolve();
    await Promise.resolve();

    // New WS is now waiting for open
    const ws2 = latestWS();
    expect(ws2).not.toBe(ws1);

    // ws1 listeners must already be removed (detachListeners called before new WS)
    expect(ws1.listenerCount('message')).toBe(0);
    expect(ws1.listenerCount('close')).toBe(0);

    ws2.triggerOpen();
    await Promise.resolve();

    // ws2 should have exactly 1 of each
    expect(ws2.listenerCount('message')).toBe(1);
    expect(ws2.listenerCount('close')).toBe(1);

    client.close();
  });

  it('removes listeners from WS on explicit close()', async () => {
    const client = new SignalingClient({ url: 'ws://localhost', token: 'tok' });
    await connectAndOpen(client);

    const ws = latestWS();
    client.close();

    expect(ws.listenerCount('message')).toBe(0);
    expect(ws.listenerCount('close')).toBe(0);
  });
});

describe('SignalingClient — token provider', () => {
  it('uses tokenProvider on first connect', async () => {
    const tokenProvider = vi.fn().mockResolvedValue('fresh-token');
    const client = new SignalingClient({ url: 'ws://localhost', token: 'static', tokenProvider });
    await connectAndOpen(client);

    expect(tokenProvider).toHaveBeenCalledTimes(1);
    expect(latestWS().url).toContain('fresh-token');
    client.close();
  });

  it('calls tokenProvider again on reconnect', async () => {
    let callCount = 0;
    const tokenProvider = vi.fn().mockImplementation(() => `token-${++callCount}`);

    const client = new SignalingClient({
      url: 'ws://localhost',
      token: 'static',
      tokenProvider,
      maxReconnectAttempts: 3,
    });
    await connectAndOpen(client);
    expect(tokenProvider).toHaveBeenCalledTimes(1);

    // Trigger reconnect
    latestWS().triggerClose();
    await vi.advanceTimersByTimeAsync(1500);
    await Promise.resolve();
    await Promise.resolve();

    // tokenProvider called for new connect attempt
    expect(tokenProvider).toHaveBeenCalledTimes(2);
    expect(latestWS().url).toContain('token-2');
    client.close();
  });
});

describe('SignalingClient — auth rejection (close code 4001)', () => {
  it('emits unauthorized and stops retrying on code 4001', async () => {
    const client = new SignalingClient({ url: 'ws://localhost', token: 'expired' });
    const errors: AftertalkError[] = [];
    const disconnectReasons: string[] = [];

    client.on('error', (e) => errors.push(e));
    client.on('disconnected', (r) => disconnectReasons.push(r));

    await connectAndOpen(client);
    const wsBefore = MockWebSocket.instances.length;

    // Server closes with 4001 (unauthorized)
    latestWS()._dispatch('close', { code: 4001, reason: '', wasClean: false } as CloseEvent);

    expect(errors).toHaveLength(1);
    expect(errors[0].code).toBe('unauthorized');
    expect(disconnectReasons).toContain('unauthorized');
    // No new WS should be created
    expect(MockWebSocket.instances).toHaveLength(wsBefore);
  });
});

describe('SignalingClient — message queue', () => {
  it('queues messages while disconnected and flushes on reconnect', async () => {
    const client = new SignalingClient({ url: 'ws://localhost', token: 'tok' });
    await connectAndOpen(client);

    latestWS().triggerClose();
    client.send({ type: 'offer', sdp: 'queued-sdp' });

    await vi.advanceTimersByTimeAsync(1500);
    await Promise.resolve();
    await Promise.resolve();

    const ws2 = latestWS();
    const sendSpy = vi.spyOn(ws2, 'send');
    ws2.triggerOpen();
    await Promise.resolve();

    expect(sendSpy).toHaveBeenCalledWith(expect.stringContaining('queued-sdp'));
    client.close();
  });
});

describe('SignalingClient — message routing', () => {
  it('emits answer event', async () => {
    const client = new SignalingClient({ url: 'ws://localhost', token: 'tok' });
    await connectAndOpen(client);
    const answers: unknown[] = [];
    client.on('answer', (m) => answers.push(m));

    latestWS().triggerMessage({ type: 'answer', sdp: 'answer-sdp' });
    expect(answers).toHaveLength(1);
    client.close();
  });

  it('emits ice-candidate event', async () => {
    const client = new SignalingClient({ url: 'ws://localhost', token: 'tok' });
    await connectAndOpen(client);
    const candidates: unknown[] = [];
    client.on('ice-candidate', (m) => candidates.push(m));

    latestWS().triggerMessage({ type: 'ice-candidate', candidate: { candidate: 'cand' } });
    expect(candidates).toHaveLength(1);
    client.close();
  });

  it('ignores pong messages', async () => {
    const client = new SignalingClient({ url: 'ws://localhost', token: 'tok' });
    await connectAndOpen(client);
    const messages: unknown[] = [];
    client.on('message', (m) => messages.push(m));

    latestWS().triggerMessage({ type: 'pong' });
    expect(messages).toHaveLength(0);
    client.close();
  });
});

describe('SignalingClient — backoff jitter', () => {
  it('applies non-zero delay before reconnecting', async () => {
    const client = new SignalingClient({
      url: 'ws://localhost',
      token: 'tok',
      backoffJitter: 0.5,
      maxReconnectAttempts: 1,
    });
    await connectAndOpen(client);

    const delays: number[] = [];
    const origSetTimeout = vi.getRealSystemTime; // use to confirm fake timers active
    void origSetTimeout; // suppress unused warning
    vi.spyOn(globalThis, 'setTimeout').mockImplementationOnce((_fn, delay) => {
      if (typeof delay === 'number') delays.push(delay);
      // Don't actually schedule — we just want to capture the delay
      return 0 as unknown as ReturnType<typeof setTimeout>;
    });

    latestWS().triggerClose();

    // Delay should be base 1000ms ±50% → [500, 1500]
    expect(delays.length).toBeGreaterThan(0);
    expect(delays[0]).toBeGreaterThan(0);
    expect(delays[0]).toBeLessThanOrEqual(1500);

    client.close();
  });
});
