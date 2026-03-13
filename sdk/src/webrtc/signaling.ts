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

  private readonly url: string;
  private readonly token: string;
  private readonly maxReconnectAttempts: number;

  private listeners: {
    [K in keyof SignalingEventMap]?: Set<SignalingListener<K>>;
  } = {};

  constructor(options: { url: string; token: string; maxReconnectAttempts?: number }) {
    this.url = options.url;
    this.token = options.token;
    this.maxReconnectAttempts = options.maxReconnectAttempts ?? 5;
  }

  get connected(): boolean {
    return this._connected;
  }

  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      const wsUrl = `${this.url}?token=${encodeURIComponent(this.token)}`;
      this.ws = new WebSocket(wsUrl);

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
          reject(new AftertalkError('webrtc_connection_failed', { message: 'WebSocket connection failed', details: e }));
        }
      };

      this.ws.addEventListener('open', onOpen, { once: true });
      this.ws.addEventListener('error', onError, { once: true });
      this.ws.addEventListener('message', this.onMessage);
      this.ws.addEventListener('close', this.onClose);
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
    this.ws?.close();
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

    if (this._closed) {
      this.emit('disconnected', 'closed');
      return;
    }

    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      this.emit('disconnected', `max reconnect attempts (${this.maxReconnectAttempts}) reached`);
      this.emit('error', new AftertalkError('signaling_reconnect_failed'));
      return;
    }

    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30_000);
    this.reconnectAttempts++;
    this.emit('reconnecting', this.reconnectAttempts);

    this.reconnectTimer = setTimeout(async () => {
      try {
        await this.connect();
      } catch {
        this.onClose(event);
      }
    }, delay);
  };

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
