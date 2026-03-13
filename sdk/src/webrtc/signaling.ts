import { AftertalkError } from '../errors.js';

// ─── Message types ────────────────────────────────────────────────────────────

export type SignalingMessageType =
  | 'offer'
  | 'answer'
  | 'ice-candidate'
  | 'error'
  | 'ping'
  | 'pong';

export interface SignalingMessage {
  type: SignalingMessageType;
  [key: string]: unknown;
}

export interface ICECandidateMessage extends SignalingMessage {
  type: 'ice-candidate';
  candidate: RTCIceCandidateInit;
}

export interface OfferMessage extends SignalingMessage {
  type: 'offer';
  sdp: string;
}

export interface AnswerMessage extends SignalingMessage {
  type: 'answer';
  sdp: string;
}

// ─── Options ─────────────────────────────────────────────────────────────────

export interface SignalingClientOptions {
  url: string;
  /** Static token used if tokenProvider is not given. */
  token: string;
  /** Max number of reconnect attempts before giving up (default: 5). */
  maxReconnectAttempts?: number;
  /**
   * Optional callback invoked on each connection attempt to obtain a fresh
   * token. Use this when tokens are short-lived (e.g. JWT with 1h TTL) so
   * that reconnects don't fail with 401.
   */
  tokenProvider?: () => string | Promise<string>;
  /**
   * Fractional jitter applied to the backoff delay (0–1, default: 0.3).
   * A value of 0.3 means ±30% random variation, which prevents thundering
   * herd when many clients reconnect simultaneously.
   */
  backoffJitter?: number;
}

// ─── Events ───────────────────────────────────────────────────────────────────

type SignalingEventMap = {
  connected: [];
  disconnected: [reason: string];
  reconnecting: [attempt: number];
  message: [msg: SignalingMessage];
  answer: [msg: AnswerMessage];
  'ice-candidate': [msg: ICECandidateMessage];
  error: [err: AftertalkError];
};

type SignalingListener<K extends keyof SignalingEventMap> = (
  ...args: SignalingEventMap[K]
) => void;

// ─── SignalingClient ──────────────────────────────────────────────────────────

export class SignalingClient {
  private ws?: WebSocket;
  private _connected = false;
  private _closed = false;
  private reconnectAttempts = 0;
  private reconnectTimer?: ReturnType<typeof setTimeout>;
  private pingTimer?: ReturnType<typeof setInterval>;
  private messageQueue: SignalingMessage[] = [];

  private readonly options: Required<Omit<SignalingClientOptions, 'tokenProvider'>> & {
    tokenProvider?: () => string | Promise<string>;
  };

  private listeners: {
    [K in keyof SignalingEventMap]?: Set<SignalingListener<K>>;
  } = {};

  constructor(options: SignalingClientOptions) {
    this.options = {
      url: options.url,
      token: options.token,
      maxReconnectAttempts: options.maxReconnectAttempts ?? 5,
      tokenProvider: options.tokenProvider,
      backoffJitter: options.backoffJitter ?? 0.3,
    };
  }

  get connected(): boolean {
    return this._connected;
  }

  async connect(): Promise<void> {
    // Detach listeners from previous WS instance to prevent accumulation.
    if (this.ws) {
      this.detachListeners(this.ws);
    }

    const token = await this.resolveToken();
    const wsUrl = `${this.options.url}?token=${encodeURIComponent(token)}`;

    return new Promise((resolve, reject) => {
      const ws = new WebSocket(wsUrl);
      this.ws = ws;

      const onOpen = () => {
        this._connected = true;
        this.reconnectAttempts = 0;
        this.flushQueue();
        this.startPing();
        this.emit('connected');
        resolve();
      };

      const onError = (e: Event) => {
        if (!this._connected) {
          reject(
            new AftertalkError('webrtc_connection_failed', {
              message: 'WebSocket connection failed',
              details: e,
            }),
          );
        }
      };

      ws.addEventListener('open', onOpen, { once: true });
      ws.addEventListener('error', onError, { once: true });
      this.attachListeners(ws);
    });
  }

  send(message: SignalingMessage): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    } else {
      this.messageQueue.push(message);
    }
  }

  close(): void {
    this._closed = true;
    this.stopPing();
    clearTimeout(this.reconnectTimer);
    if (this.ws) {
      this.detachListeners(this.ws);
      this.ws.close();
    }
    this._connected = false;
  }

  on<K extends keyof SignalingEventMap>(event: K, listener: SignalingListener<K>): this {
    if (!this.listeners[event]) {
      this.listeners[event] = new Set() as never;
    }
    (this.listeners[event] as Set<SignalingListener<K>>).add(listener);
    return this;
  }

  off<K extends keyof SignalingEventMap>(event: K, listener: SignalingListener<K>): this {
    (this.listeners[event] as Set<SignalingListener<K>> | undefined)?.delete(listener);
    return this;
  }

  private emit<K extends keyof SignalingEventMap>(event: K, ...args: SignalingEventMap[K]): void {
    const set = this.listeners[event] as Set<SignalingListener<K>> | undefined;
    if (set) {
      for (const listener of set) {
        try {
          (listener as (...a: SignalingEventMap[K]) => void)(...args);
        } catch {
          // swallow listener errors
        }
      }
    }
  }

  // Attach persistent (non-once) listeners to a WebSocket instance.
  private attachListeners(ws: WebSocket): void {
    ws.addEventListener('message', this.onMessage);
    ws.addEventListener('close', this.onClose);
  }

  // Remove persistent listeners — must be called before replacing this.ws.
  private detachListeners(ws: WebSocket): void {
    ws.removeEventListener('message', this.onMessage);
    ws.removeEventListener('close', this.onClose);
  }

  private onMessage = (event: MessageEvent): void => {
    let msg: SignalingMessage;
    try {
      msg = JSON.parse(event.data as string) as SignalingMessage;
    } catch {
      return;
    }

    if (msg.type === 'pong') return;

    this.emit('message', msg);

    if (msg.type === 'answer') {
      this.emit('answer', msg as AnswerMessage);
    } else if (msg.type === 'ice-candidate') {
      this.emit('ice-candidate', msg as ICECandidateMessage);
    } else if (msg.type === 'error') {
      const err = new AftertalkError('webrtc_connection_failed', {
        message: typeof msg['message'] === 'string' ? msg['message'] : 'Signaling error',
      });
      this.emit('error', err);
    }
  };

  private onClose = (event: CloseEvent): void => {
    this._connected = false;
    this.stopPing();

    // WS close code 4001 → treat as auth failure rather than retriable error.
    if (event.code === 4001 || event.code === 4003) {
      this.emit('disconnected', 'unauthorized');
      this.emit('error', new AftertalkError('unauthorized', { message: 'Signaling token rejected' }));
      return;
    }

    if (this._closed) {
      this.emit('disconnected', 'closed');
      return;
    }

    if (this.reconnectAttempts >= this.options.maxReconnectAttempts) {
      this.emit(
        'disconnected',
        `max reconnect attempts (${this.options.maxReconnectAttempts}) reached`,
      );
      this.emit('error', new AftertalkError('signaling_reconnect_failed'));
      return;
    }

    const delay = this.backoffDelay();
    this.reconnectAttempts++;
    this.emit('reconnecting', this.reconnectAttempts);

    // Non-async callback: if we used `async () => { await this.connect() }`,
    // advanceTimersByTimeAsync() would await the returned Promise — which
    // hangs until the WS open event fires, deadlocking tests. Using a
    // sync wrapper with void ensures the timer returns undefined immediately.
    this.reconnectTimer = setTimeout(() => {
      void this.connect().catch((err: unknown) => {
        if (err instanceof AftertalkError && err.code === 'unauthorized') {
          this.emit('disconnected', 'unauthorized');
          this.emit('error', err);
          return;
        }
        this.onClose(event);
      });
    }, delay);
  };

  private async resolveToken(): Promise<string> {
    if (this.options.tokenProvider) {
      return await this.options.tokenProvider();
    }
    return this.options.token;
  }

  private backoffDelay(): number {
    const base = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30_000);
    const jitter = this.options.backoffJitter;
    // Apply symmetric jitter: base * (1 ± jitter)
    const factor = 1 - jitter + Math.random() * jitter * 2;
    return Math.round(base * factor);
  }

  private flushQueue(): void {
    while (this.messageQueue.length > 0) {
      const msg = this.messageQueue.shift();
      if (msg) this.send(msg);
    }
  }

  private startPing(): void {
    this.pingTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'ping' }));
      }
    }, 20_000);
  }

  private stopPing(): void {
    clearInterval(this.pingTimer);
  }
}
