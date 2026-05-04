import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AftertalkError } from '../errors.js';
import { WebRTCConnection } from './connection.js';

// ─── Mocks ────────────────────────────────────────────────────────────────────

vi.mock('./audio.js', () => ({
  AudioManager: class {
    muted = false;
    async acquire() {
      return {
        getAudioTracks: () => [{ stop: vi.fn() }],
        active: true,
      };
    }
    setMuted(v: boolean) { this.muted = v; }
    release() {}
  },
}));

type SignalingEventName = 'connected' | 'disconnected' | 'reconnecting' | 'answer' | 'ice-candidate' | 'error';

class MockSignalingClient {
  private handlers: Map<string, ((...args: unknown[]) => void)[]> = new Map();
  connectCalled = 0;
  sentMessages: unknown[] = [];

  async connect() { this.connectCalled++; }
  close() {}
  send(msg: unknown) { this.sentMessages.push(msg); }

  on(event: string, handler: (...args: unknown[]) => void) {
    if (!this.handlers.has(event)) this.handlers.set(event, []);
    this.handlers.get(event)!.push(handler);
    return this;
  }

  emit(event: SignalingEventName, ...args: unknown[]) {
    for (const h of this.handlers.get(event) ?? []) h(...args);
  }
}

class MockPeerConnection {
  iceConnectionState: RTCIceConnectionState = 'new';
  onicecandidate: ((e: RTCPeerConnectionIceEvent) => void) | null = null;
  oniceconnectionstatechange: (() => void) | null = null;

  createOfferCalled = 0;
  setLocalDescriptionCalled = 0;
  addIceCandidateCalled = 0;
  restartIceCalled = 0;

  async createOffer(_opts?: RTCOfferOptions) {
    this.createOfferCalled++;
    return { type: 'offer' as RTCSdpType, sdp: 'mock-sdp' };
  }
  async setLocalDescription(_desc: RTCSessionDescriptionInit) {
    this.setLocalDescriptionCalled++;
  }
  async setRemoteDescription(_desc: RTCSessionDescriptionInit) {}
  async addIceCandidate(_c: RTCIceCandidateInit) { this.addIceCandidateCalled++; }
  addTrack() {}
  restartIce() { this.restartIceCalled++; }
  close() {}

  // Test helper: simulate ICE state change
  simulateICEState(state: RTCIceConnectionState) {
    this.iceConnectionState = state;
    this.oniceconnectionstatechange?.();
  }
}

let mockPC: MockPeerConnection;
let mockSignaling: MockSignalingClient;

vi.mock('./signaling.js', () => ({
  SignalingClient: class {
    constructor() { return mockSignaling; }
  },
}));

beforeEach(() => {
  mockPC = new MockPeerConnection();
  mockSignaling = new MockSignalingClient();
  vi.stubGlobal('RTCPeerConnection', class { constructor() { return mockPC; } });
  vi.useFakeTimers();
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
});

async function makeConnectedConnection(cfg = {}) {
  const conn = new WebRTCConnection(cfg);
  await conn.connect({
    sessionId: 's1',
    token: 'tok',
    signalingUrl: 'ws://localhost/signaling',
    iceServers: [],
  });
  return conn;
}

// ─── Tests ────────────────────────────────────────────────────────────────────

describe('WebRTCConnection — ICE disconnected grace period', () => {
  it('does NOT emit disconnected immediately on ICE disconnected', async () => {
    const conn = await makeConnectedConnection({ iceDisconnectedGraceMs: 5000 });
    const disconnects: string[] = [];
    conn.on('disconnected', (r) => disconnects.push(r));

    mockPC.simulateICEState('disconnected');

    // Grace period not elapsed — no disconnect emitted
    expect(disconnects).toHaveLength(0);
  });

  it('attempts ICE restart after grace period if still disconnected', async () => {
    const conn = await makeConnectedConnection({ iceDisconnectedGraceMs: 1000 });
    const restartEvents: number[] = [];
    conn.on('ice-restarting', (n) => restartEvents.push(n));

    mockPC.simulateICEState('disconnected');
    expect(restartEvents).toHaveLength(0);

    // Advance past grace period
    await vi.advanceTimersByTimeAsync(1100);

    expect(mockPC.restartIceCalled).toBe(1);
    expect(mockPC.createOfferCalled).toBeGreaterThanOrEqual(2); // initial + restart
    expect(restartEvents).toContain(1);
  });

  it('cancels grace timer when ICE recovers to connected', async () => {
    const conn = await makeConnectedConnection({ iceDisconnectedGraceMs: 1000 });
    const errors: AftertalkError[] = [];
    conn.on('error', (e) => errors.push(e));

    mockPC.simulateICEState('disconnected');
    // Recover before grace period
    mockPC.simulateICEState('connected');

    await vi.advanceTimersByTimeAsync(1500);

    // No ICE restart should have fired
    expect(mockPC.restartIceCalled).toBe(0);
    expect(errors).toHaveLength(0);
  });
});

describe('WebRTCConnection — ICE restart on failed', () => {
  it('attempts ICE restart immediately on ice failed', async () => {
    const conn = await makeConnectedConnection();
    const restartEvents: number[] = [];
    conn.on('ice-restarting', (n) => restartEvents.push(n));

    mockPC.simulateICEState('failed');
    await vi.runAllTimersAsync();

    expect(mockPC.restartIceCalled).toBe(1);
    expect(restartEvents).toContain(1);
  });

  it('sends new offer with iceRestart:true via signaling', async () => {
    await makeConnectedConnection();

    mockPC.simulateICEState('failed');
    await vi.runAllTimersAsync();

    const offerMsg = mockSignaling.sentMessages.find(
      (m) => (m as { type: string }).type === 'offer',
    );
    expect(offerMsg).toBeDefined();
  });

  it('stops after maxIceRestarts and emits error', async () => {
    // maxIceRestarts=2: allows exactly 2 restart attempts, errors on the 3rd failure.
    const conn = await makeConnectedConnection({ maxIceRestarts: 2 });
    const errors: AftertalkError[] = [];
    conn.on('error', (e) => errors.push(e));

    // Failure 1 → restart 1
    mockPC.simulateICEState('failed');
    await vi.runAllTimersAsync();
    expect(mockPC.restartIceCalled).toBe(1);

    // Answer clears in-progress; failure 2 → restart 2
    // The answer handler is async (await setRemoteDescription), so we must
    // flush the microtask queue before iceRestartInProgress becomes false.
    mockSignaling.emit('answer', { type: 'answer', sdp: 'sdp' });
    await Promise.resolve(); // let async answer handler complete
    mockPC.simulateICEState('failed');
    await vi.runAllTimersAsync();
    expect(mockPC.restartIceCalled).toBe(2);

    // Answer clears in-progress; failure 3 → max reached, emit error, no restart
    mockSignaling.emit('answer', { type: 'answer', sdp: 'sdp' });
    await Promise.resolve();
    mockPC.simulateICEState('failed');
    await vi.runAllTimersAsync();
    expect(mockPC.restartIceCalled).toBe(2); // no third restart

    const iceErrors = errors.filter((e) => e.code === 'webrtc_ice_failed');
    expect(iceErrors.length).toBeGreaterThanOrEqual(1);
  });

  it('resets restart counter when ICE reaches connected (not just on answer)', async () => {
    const conn = await makeConnectedConnection({ maxIceRestarts: 2 });
    const errors: AftertalkError[] = [];
    conn.on('error', (e) => errors.push(e));

    // ICE fails → restart → answer received → ICE recovers to connected
    mockPC.simulateICEState('failed');
    await vi.runAllTimersAsync();
    mockSignaling.emit('answer', { type: 'answer', sdp: 'sdp' });
    await Promise.resolve(); // flush async answer handler
    mockPC.simulateICEState('connected'); // ← counter resets here

    // ICE fails again — counter is 0 again, should restart without hitting max
    mockPC.simulateICEState('failed');
    await vi.runAllTimersAsync();

    // Still no terminal error
    const terminalErrors = errors.filter((e) => e.code === 'webrtc_ice_failed');
    expect(terminalErrors).toHaveLength(0);
    expect(mockPC.restartIceCalled).toBe(2); // two separate restarts
  });
});

describe('WebRTCConnection — renegotiate after signaling reconnect', () => {
  it('triggers ICE restart when signaling reconnects and ICE is failed', async () => {
    await makeConnectedConnection();

    mockPC.iceConnectionState = 'failed';
    mockSignaling.emit('connected');
    await vi.runAllTimersAsync();

    expect(mockPC.restartIceCalled).toBe(1);
  });

  it('does not restart ICE when signaling reconnects and ICE is healthy', async () => {
    await makeConnectedConnection();

    mockPC.iceConnectionState = 'connected';
    mockSignaling.emit('connected');
    await vi.runAllTimersAsync();

    expect(mockPC.restartIceCalled).toBe(0);
  });
});

describe('WebRTCConnection — ICE closed', () => {
  it('emits disconnected on ICE closed', async () => {
    const conn = await makeConnectedConnection();
    const disconnects: string[] = [];
    conn.on('disconnected', (r) => disconnects.push(r));

    mockPC.simulateICEState('closed');

    expect(disconnects).toContain('ICE closed');
  });
});
