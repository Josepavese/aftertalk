import { AftertalkError } from '../errors.js';
import type { ICEServer, WebRTCConfig } from '../types.js';
import { AudioManager } from './audio.js';
import { SignalingClient } from './signaling.js';

// ─── Events ───────────────────────────────────────────────────────────────────

type ConnectionEventMap = {
  connected: [sessionId: string];
  disconnected: [reason: string];
  'audio-started': [];
  'ice-state-changed': [state: RTCIceConnectionState];
  'signaling-reconnecting': [attempt: number];
  error: [err: AftertalkError];
};

type ConnectionListener<K extends keyof ConnectionEventMap> = (
  ...args: ConnectionEventMap[K]
) => void;

// ─── WebRTCConnection ─────────────────────────────────────────────────────────

export class WebRTCConnection {
  private pc?: RTCPeerConnection;
  private signaling?: SignalingClient;
  private audio: AudioManager;
  private _sessionId?: string;

  private listeners: {
    [K in keyof ConnectionEventMap]?: Set<ConnectionListener<K>>;
  } = {};

  constructor(private readonly config: WebRTCConfig = {}) {
    this.audio = new AudioManager();
  }

  get sessionId(): string | undefined {
    return this._sessionId;
  }

  get muted(): boolean {
    return this.audio.muted;
  }

  async connect(options: {
    sessionId: string;
    token: string;
    signalingUrl: string;
    iceServers: ICEServer[];
  }): Promise<void> {
    const { sessionId, token, signalingUrl, iceServers } = options;
    this._sessionId = sessionId;

    // 1. Acquire microphone
    const stream = await this.audio.acquire(this.config.audioConstraints);
    this.emit('audio-started');

    // 2. Create peer connection
    this.pc = new RTCPeerConnection({ iceServers: iceServers as RTCIceServer[] });
    this.setupPCListeners();

    // 3. Add audio track
    for (const track of stream.getAudioTracks()) {
      this.pc.addTrack(track, stream);
    }

    // 4. Connect signaling WS
    this.signaling = new SignalingClient({
      url: signalingUrl,
      token,
      maxReconnectAttempts: this.config.maxReconnectAttempts,
    });

    this.signaling.on('answer', async (msg) => {
      if (!this.pc) return;
      await this.pc.setRemoteDescription({ type: 'answer', sdp: msg.sdp });
    });

    this.signaling.on('ice-candidate', async (msg) => {
      if (!this.pc) return;
      try {
        await this.pc.addIceCandidate(msg.candidate);
      } catch {
        // non-fatal: stale candidate after reconnect
      }
    });

    this.signaling.on('disconnected', (reason) => this.emit('disconnected', reason));
    this.signaling.on('reconnecting', (attempt) => this.emit('signaling-reconnecting', attempt));
    this.signaling.on('error', (err) => this.emit('error', err));

    await this.signaling.connect();

    // 5. Create and send offer
    const offer = await this.pc.createOffer();
    await this.pc.setLocalDescription(offer);
    this.signaling.send({ type: 'offer', sdp: offer.sdp ?? '' });
  }

  setMuted(muted: boolean): void {
    this.audio.setMuted(muted);
  }

  async disconnect(): Promise<void> {
    this.signaling?.close();
    this.pc?.close();
    this.audio.release();
    this.pc = undefined;
    this.signaling = undefined;
    this._sessionId = undefined;
    this.emit('disconnected', 'closed');
  }

  on<K extends keyof ConnectionEventMap>(event: K, listener: ConnectionListener<K>): this {
    if (!this.listeners[event]) {
      this.listeners[event] = new Set() as never;
    }
    (this.listeners[event] as Set<ConnectionListener<K>>).add(listener);
    return this;
  }

  off<K extends keyof ConnectionEventMap>(event: K, listener: ConnectionListener<K>): this {
    (this.listeners[event] as Set<ConnectionListener<K>> | undefined)?.delete(listener);
    return this;
  }

  private setupPCListeners(): void {
    if (!this.pc) return;

    this.pc.onicecandidate = (event) => {
      if (event.candidate) {
        this.signaling?.send({ type: 'ice-candidate', candidate: event.candidate.toJSON() });
      }
    };

    this.pc.oniceconnectionstatechange = () => {
      if (!this.pc) return;
      const state = this.pc.iceConnectionState;
      this.emit('ice-state-changed', state);

      if (state === 'connected' || state === 'completed') {
        this.emit('connected', this._sessionId ?? '');
      } else if (state === 'failed') {
        this.emit('error', new AftertalkError('webrtc_ice_failed', { message: 'ICE connection failed' }));
      } else if (state === 'disconnected' || state === 'closed') {
        this.emit('disconnected', `ICE ${state}`);
      }
    };
  }

  private emit<K extends keyof ConnectionEventMap>(event: K, ...args: ConnectionEventMap[K]): void {
    const set = this.listeners[event] as Set<ConnectionListener<K>> | undefined;
    if (set) {
      for (const listener of set) {
        try {
          (listener as (...a: ConnectionEventMap[K]) => void)(...args);
        } catch {
          // swallow
        }
      }
    }
  }
}
