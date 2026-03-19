// src/api/config.ts
var ConfigAPI = class {
  constructor(http) {
    this.http = http;
  }
  /** Public endpoint — no API key required. Returns templates, default_template_id, and provider profiles. */
  async getConfig() {
    const raw = await this.http.get("/v1/config");
    return {
      templates: raw.templates,
      defaultTemplateId: raw.default_template_id,
      sttProfiles: raw.stt_profiles,
      llmProfiles: raw.llm_profiles,
      sttDefaultProfile: raw.default_stt_profile,
      llmDefaultProfile: raw.default_llm_profile
    };
  }
  /** Returns ICE server list for WebRTC, optionally with HMAC credentials. */
  async getRTCConfig() {
    const raw = await this.http.get("/v1/rtc-config");
    return {
      iceServers: raw.ice_servers ?? [],
      ttl: raw.ttl
    };
  }
};

// src/api/minutes.ts
var MinutesAPI = class {
  constructor(http) {
    this.http = http;
  }
  /** GET /v1/minutes?session_id={sessionId} */
  async getBySession(sessionId) {
    return this.http.get(`/v1/minutes?session_id=${encodeURIComponent(sessionId)}`);
  }
  /** GET /v1/minutes/{minutesId} */
  async get(minutesId) {
    return this.http.get(`/v1/minutes/${minutesId}`);
  }
  /** PUT /v1/minutes/{minutesId}  — header X-User-Id to track the editor */
  async update(minutesId, request, userId) {
    const headers = userId ? { "X-User-Id": userId } : {};
    return this.http.put(`/v1/minutes/${minutesId}`, request, { headers });
  }
  /** GET /v1/minutes/{minutesId}/versions */
  async getVersions(minutesId) {
    return this.http.get(`/v1/minutes/${minutesId}/versions`);
  }
  /** DELETE /v1/minutes/{minutesId} */
  async delete(minutesId) {
    return this.http.delete(`/v1/minutes/${minutesId}`);
  }
};

// src/api/rooms.ts
var RoomsAPI = class {
  constructor(http) {
    this.http = http;
  }
  /**
   * Join or create a room session by code.
   * Creates the session the first time; subsequent participants get their own token.
   * Role is exclusive: two participants cannot share the same role.
   */
  async join(request) {
    const raw = await this.http.post(
      "/v1/rooms/join",
      {
        code: request.code,
        name: request.name,
        role: request.role,
        template_id: request.templateId,
        stt_profile: request.sttProfile,
        llm_profile: request.llmProfile
      }
    );
    return { sessionId: raw.session_id, token: raw.token };
  }
};

// src/api/sessions.ts
var SessionsAPI = class {
  constructor(http) {
    this.http = http;
  }
  async create(request) {
    return this.http.post("/v1/sessions", request);
  }
  async get(sessionId) {
    return this.http.get(`/v1/sessions/${sessionId}`);
  }
  async getStatus(sessionId) {
    return this.http.get(`/v1/sessions/${sessionId}/status`);
  }
  async end(sessionId) {
    return this.http.post(`/v1/sessions/${sessionId}/end`);
  }
  async list(filters) {
    const params = new URLSearchParams();
    if (filters?.status) params.set("status", filters.status);
    if (filters?.limit !== void 0) params.set("limit", String(filters.limit));
    if (filters?.offset !== void 0) params.set("offset", String(filters.offset));
    const qs = params.toString();
    return this.http.get(`/v1/sessions${qs ? `?${qs}` : ""}`);
  }
  async delete(sessionId) {
    return this.http.delete(`/v1/sessions/${sessionId}`);
  }
};

// src/api/transcriptions.ts
var TranscriptionsAPI = class {
  constructor(http) {
    this.http = http;
  }
  /** GET /v1/transcriptions?session_id={sessionId} */
  async listBySession(sessionId, filters) {
    const params = new URLSearchParams({ session_id: sessionId });
    if (filters?.limit !== void 0) params.set("limit", String(filters.limit));
    if (filters?.offset !== void 0) params.set("offset", String(filters.offset));
    return this.http.get(`/v1/transcriptions?${params}`);
  }
  /** GET /v1/transcriptions/{transcriptionId} */
  async get(transcriptionId) {
    return this.http.get(`/v1/transcriptions/${transcriptionId}`);
  }
};

// src/errors.ts
var AftertalkError = class _AftertalkError extends Error {
  constructor(code, options) {
    super(options?.message ?? code);
    this.name = "AftertalkError";
    this.code = code;
    this.status = options?.status;
    this.details = options?.details;
  }
  static fromHttpStatus(status, body) {
    const details = body;
    const message = extractMessage(body);
    switch (status) {
      case 400:
        return new _AftertalkError("bad_request", { status, message, details });
      case 401:
        return new _AftertalkError("unauthorized", { status, message, details });
      case 403:
        return new _AftertalkError("forbidden", { status, message, details });
      case 404:
        return new _AftertalkError("not_found", { status, message, details });
      case 409:
        return new _AftertalkError("conflict", { status, message, details });
      case 429:
        return new _AftertalkError("rate_limited", { status, message, details });
      default:
        if (status >= 500) {
          return new _AftertalkError("server_error", { status, message, details });
        }
        return new _AftertalkError("unknown", { status, message, details });
    }
  }
};
function extractMessage(body) {
  if (!body || typeof body !== "object") return void 0;
  const b = body;
  return typeof b["error"] === "string" ? b["error"] : typeof b["message"] === "string" ? b["message"] : void 0;
}

// src/http.ts
var HttpClient = class {
  constructor(options) {
    this.baseUrl = options.baseUrl.replace(/\/$/, "");
    this.apiKey = options.apiKey;
    this.timeout = options.timeout ?? 3e4;
    this.fetchImpl = options.fetch ?? globalThis.fetch.bind(globalThis);
  }
  async get(path, options) {
    return this.request(path, { ...options, method: "GET" });
  }
  async post(path, body, options) {
    return this.request(path, { ...options, method: "POST", body });
  }
  async put(path, body, options) {
    return this.request(path, { ...options, method: "PUT", body });
  }
  async delete(path, options) {
    return this.request(path, { ...options, method: "DELETE" });
  }
  async request(path, options = {}) {
    const { method = "GET", body, headers = {}, signal } = options;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);
    const combinedSignal = signal ? anySignal([signal, controller.signal]) : controller.signal;
    const reqHeaders = {
      "Content-Type": "application/json",
      "Accept": "application/json",
      ...headers
    };
    if (this.apiKey) {
      reqHeaders["X-API-Key"] = this.apiKey;
    }
    let response;
    try {
      response = await this.fetchImpl(`${this.baseUrl}${path}`, {
        method,
        headers: reqHeaders,
        body: body !== void 0 ? JSON.stringify(body) : void 0,
        signal: combinedSignal
      });
    } catch (err) {
      if (err instanceof Error && err.name === "AbortError") {
        throw new AftertalkError("timeout", { message: `Request timed out after ${this.timeout}ms` });
      }
      throw new AftertalkError("network_error", {
        message: err instanceof Error ? err.message : "Network request failed",
        details: err
      });
    } finally {
      clearTimeout(timeoutId);
    }
    if (response.status === 204) {
      return void 0;
    }
    let responseBody;
    const contentType = response.headers.get("content-type") ?? "";
    if (contentType.includes("application/json")) {
      responseBody = await response.json().catch(() => null);
    } else {
      responseBody = await response.text().catch(() => null);
    }
    if (!response.ok) {
      throw AftertalkError.fromHttpStatus(response.status, responseBody);
    }
    return responseBody;
  }
};
function anySignal(signals) {
  const controller = new AbortController();
  const onAbort = () => controller.abort();
  for (const signal of signals) {
    if (signal.aborted) {
      controller.abort();
      break;
    }
    signal.addEventListener("abort", onAbort, { once: true });
  }
  controller.signal.addEventListener("abort", () => {
    for (const signal of signals) {
      signal.removeEventListener("abort", onAbort);
    }
  }, { once: true });
  return controller.signal;
}

// src/realtime/minutes-poller.ts
function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
var MinutesPoller = class {
  constructor(api) {
    this.api = api;
  }
  /**
   * Polls until minutes reach `ready` or `delivered` status,
   * using exponential backoff between attempts.
   */
  async waitForReady(sessionId, options = {}) {
    const {
      timeout = 12e4,
      minInterval = 2e3,
      maxInterval = 3e4,
      backoffFactor = 1.5
    } = options;
    const deadline = Date.now() + timeout;
    let interval = minInterval;
    while (Date.now() < deadline) {
      const minutes = await this.api.getBySession(sessionId);
      if (minutes.status === "ready" || minutes.status === "delivered") {
        return minutes;
      }
      if (minutes.status === "error") {
        throw new AftertalkError("minutes_generation_failed", {
          message: "Minutes generation failed on server",
          details: minutes
        });
      }
      const remaining = deadline - Date.now();
      if (remaining <= 0) break;
      await sleep(Math.min(interval, remaining));
      interval = Math.min(interval * backoffFactor, maxInterval);
    }
    throw new AftertalkError("minutes_polling_timeout", {
      message: `Minutes not ready after ${timeout}ms`,
      details: { sessionId, timeout }
    });
  }
  /**
   * Polls with a callback fired each time a new version is detected.
   * Resolves when the session is completed or on timeout.
   */
  async watch(sessionId, onUpdate, options = {}) {
    const {
      timeout = 3e5,
      minInterval = 5e3,
      maxInterval = 3e4,
      backoffFactor = 1.5
    } = options;
    const deadline = Date.now() + timeout;
    let interval = minInterval;
    let lastVersion = -1;
    while (Date.now() < deadline) {
      try {
        const minutes = await this.api.getBySession(sessionId);
        if (minutes.version !== lastVersion) {
          lastVersion = minutes.version;
          onUpdate(minutes);
        }
        if (minutes.status === "delivered") return;
        if (minutes.status === "error") {
          throw new AftertalkError("minutes_generation_failed", { details: minutes });
        }
      } catch (err) {
        if (err instanceof AftertalkError && err.code === "not_found") ; else {
          throw err;
        }
      }
      const remaining = deadline - Date.now();
      if (remaining <= 0) break;
      await sleep(Math.min(interval, remaining));
      interval = Math.min(interval * backoffFactor, maxInterval);
    }
  }
};

// src/webrtc/audio.ts
var AudioManager = class {
  constructor() {
    this._muted = false;
  }
  get muted() {
    return this._muted;
  }
  get active() {
    return this.stream?.active ?? false;
  }
  async acquire(constraints) {
    this.release();
    const audioConstraints = {
      echoCancellation: true,
      noiseSuppression: true,
      sampleRate: 48e3,
      ...constraints
    };
    try {
      this.stream = await navigator.mediaDevices.getUserMedia({ audio: audioConstraints, video: false });
      return this.stream;
    } catch (err) {
      if (err instanceof DOMException) {
        if (err.name === "NotAllowedError" || err.name === "PermissionDeniedError") {
          throw new AftertalkError("audio_permission_denied", {
            message: "Microphone access denied",
            details: err
          });
        }
        if (err.name === "NotFoundError" || err.name === "DevicesNotFoundError") {
          throw new AftertalkError("audio_device_not_found", {
            message: "No audio input device found",
            details: err
          });
        }
      }
      throw new AftertalkError("audio_permission_denied", { details: err });
    }
  }
  setMuted(muted) {
    this._muted = muted;
    if (this.stream) {
      for (const track of this.stream.getAudioTracks()) {
        track.enabled = !muted;
      }
    }
  }
  release() {
    if (this.stream) {
      for (const track of this.stream.getTracks()) {
        track.stop();
      }
      this.stream = void 0;
    }
  }
};

// src/webrtc/signaling.ts
var SignalingClient = class {
  constructor(options) {
    this._connected = false;
    this._closed = false;
    this.reconnectAttempts = 0;
    this.messageQueue = [];
    this.listeners = {};
    this.onMessage = (event) => {
      let msg;
      try {
        msg = JSON.parse(event.data);
      } catch {
        return;
      }
      if (msg.type === "pong") return;
      this.emit("message", msg);
      if (msg.type === "answer") {
        this.emit("answer", msg);
      } else if (msg.type === "ice-candidate") {
        this.emit("ice-candidate", msg);
      } else if (msg.type === "error") {
        const err = new AftertalkError("webrtc_connection_failed", {
          message: typeof msg["message"] === "string" ? msg["message"] : "Signaling error"
        });
        this.emit("error", err);
      }
    };
    this.onClose = (event) => {
      this._connected = false;
      this.stopPing();
      if (event.code === 4001 || event.code === 4003) {
        this.emit("disconnected", "unauthorized");
        this.emit("error", new AftertalkError("unauthorized", { message: "Signaling token rejected" }));
        return;
      }
      if (this._closed) {
        this.emit("disconnected", "closed");
        return;
      }
      if (this.reconnectAttempts >= this.options.maxReconnectAttempts) {
        this.emit(
          "disconnected",
          `max reconnect attempts (${this.options.maxReconnectAttempts}) reached`
        );
        this.emit("error", new AftertalkError("signaling_reconnect_failed"));
        return;
      }
      const delay = this.backoffDelay();
      this.reconnectAttempts++;
      this.emit("reconnecting", this.reconnectAttempts);
      this.reconnectTimer = setTimeout(() => {
        void this.connect().catch((err) => {
          if (err instanceof AftertalkError && err.code === "unauthorized") {
            this.emit("disconnected", "unauthorized");
            this.emit("error", err);
            return;
          }
          this.onClose(event);
        });
      }, delay);
    };
    this.options = {
      url: options.url,
      token: options.token,
      maxReconnectAttempts: options.maxReconnectAttempts ?? 5,
      tokenProvider: options.tokenProvider,
      backoffJitter: options.backoffJitter ?? 0.3
    };
  }
  get connected() {
    return this._connected;
  }
  async connect() {
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
        this.emit("connected");
        resolve();
      };
      const onError = (e) => {
        if (!this._connected) {
          reject(
            new AftertalkError("webrtc_connection_failed", {
              message: "WebSocket connection failed",
              details: e
            })
          );
        }
      };
      ws.addEventListener("open", onOpen, { once: true });
      ws.addEventListener("error", onError, { once: true });
      this.attachListeners(ws);
    });
  }
  send(message) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    } else {
      this.messageQueue.push(message);
    }
  }
  close() {
    this._closed = true;
    this.stopPing();
    clearTimeout(this.reconnectTimer);
    if (this.ws) {
      this.detachListeners(this.ws);
      this.ws.close();
    }
    this._connected = false;
  }
  on(event, listener) {
    if (!this.listeners[event]) {
      this.listeners[event] = /* @__PURE__ */ new Set();
    }
    this.listeners[event].add(listener);
    return this;
  }
  off(event, listener) {
    this.listeners[event]?.delete(listener);
    return this;
  }
  emit(event, ...args) {
    const set = this.listeners[event];
    if (set) {
      for (const listener of set) {
        try {
          listener(...args);
        } catch {
        }
      }
    }
  }
  // Attach persistent (non-once) listeners to a WebSocket instance.
  attachListeners(ws) {
    ws.addEventListener("message", this.onMessage);
    ws.addEventListener("close", this.onClose);
  }
  // Remove persistent listeners — must be called before replacing this.ws.
  detachListeners(ws) {
    ws.removeEventListener("message", this.onMessage);
    ws.removeEventListener("close", this.onClose);
  }
  async resolveToken() {
    if (this.options.tokenProvider) {
      return await this.options.tokenProvider();
    }
    return this.options.token;
  }
  backoffDelay() {
    const base = Math.min(1e3 * Math.pow(2, this.reconnectAttempts), 3e4);
    const jitter = this.options.backoffJitter;
    const factor = 1 - jitter + Math.random() * jitter * 2;
    return Math.round(base * factor);
  }
  flushQueue() {
    while (this.messageQueue.length > 0) {
      const msg = this.messageQueue.shift();
      if (msg) this.send(msg);
    }
  }
  startPing() {
    this.pingTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: "ping" }));
      }
    }, 2e4);
  }
  stopPing() {
    clearInterval(this.pingTimer);
  }
};

// src/webrtc/connection.ts
var WebRTCConnection = class {
  constructor(config = {}) {
    this.config = config;
    // ICE restart state.
    this.iceRestartAttempts = 0;
    this.iceRestartInProgress = false;
    this.listeners = {};
    this.audio = new AudioManager();
  }
  get sessionId() {
    return this._sessionId;
  }
  get muted() {
    return this.audio.muted;
  }
  async connect(options) {
    const { sessionId, token, signalingUrl, iceServers } = options;
    this._sessionId = sessionId;
    const stream = await this.audio.acquire(this.config.audioConstraints);
    this.emit("audio-started");
    this.pc = new RTCPeerConnection({ iceServers });
    this.setupPCListeners();
    for (const track of stream.getAudioTracks()) {
      this.pc.addTrack(track, stream);
    }
    this.signaling = new SignalingClient({
      url: signalingUrl,
      token,
      maxReconnectAttempts: this.config.maxReconnectAttempts,
      tokenProvider: this.config.tokenProvider,
      backoffJitter: this.config.backoffJitter
    });
    this.signaling.on("answer", async (msg) => {
      if (!this.pc) return;
      await this.pc.setRemoteDescription({ type: "answer", sdp: msg.sdp });
      this.iceRestartInProgress = false;
    });
    this.signaling.on("ice-candidate", async (msg) => {
      if (!this.pc) return;
      try {
        await this.pc.addIceCandidate(msg.candidate);
      } catch {
      }
    });
    this.signaling.on("connected", async () => {
      const iceState = this.pc?.iceConnectionState;
      if (iceState === "failed" || iceState === "disconnected") {
        await this.attemptICERestart();
      }
    });
    this.signaling.on("disconnected", (reason) => this.emit("disconnected", reason));
    this.signaling.on("reconnecting", (attempt) => this.emit("signaling-reconnecting", attempt));
    this.signaling.on("error", (err) => this.emit("error", err));
    await this.signaling.connect();
    const offer = await this.pc.createOffer();
    await this.pc.setLocalDescription(offer);
    this.signaling.send({ type: "offer", sdp: offer.sdp ?? "" });
  }
  setMuted(muted) {
    this.audio.setMuted(muted);
  }
  async disconnect() {
    this.clearICEDisconnectedTimer();
    this.signaling?.close();
    this.pc?.close();
    this.audio.release();
    this.pc = void 0;
    this.signaling = void 0;
    this._sessionId = void 0;
    this.iceRestartAttempts = 0;
    this.iceRestartInProgress = false;
    this.emit("disconnected", "closed");
  }
  on(event, listener) {
    if (!this.listeners[event]) {
      this.listeners[event] = /* @__PURE__ */ new Set();
    }
    this.listeners[event].add(listener);
    return this;
  }
  off(event, listener) {
    this.listeners[event]?.delete(listener);
    return this;
  }
  setupPCListeners() {
    if (!this.pc) return;
    this.pc.onicecandidate = (event) => {
      if (event.candidate) {
        this.signaling?.send({ type: "ice-candidate", candidate: event.candidate.toJSON() });
      }
    };
    this.pc.oniceconnectionstatechange = () => {
      if (!this.pc) return;
      const state = this.pc.iceConnectionState;
      this.emit("ice-state-changed", state);
      if (state === "connected" || state === "completed") {
        this.clearICEDisconnectedTimer();
        this.iceRestartAttempts = 0;
        this.iceRestartInProgress = false;
        this.emit("connected", this._sessionId ?? "");
      } else if (state === "disconnected") {
        const graceMs = this.config.iceDisconnectedGraceMs ?? 5e3;
        this.iceDisconnectedTimer = setTimeout(async () => {
          if (this.pc?.iceConnectionState === "disconnected") {
            await this.attemptICERestart();
          }
        }, graceMs);
      } else if (state === "failed") {
        this.clearICEDisconnectedTimer();
        void this.attemptICERestart();
      } else if (state === "closed") {
        this.clearICEDisconnectedTimer();
        this.emit("disconnected", "ICE closed");
      }
    };
  }
  /**
   * Attempts to recover a broken ICE connection via ICE restart (RFC 8445 §9.3.2).
   * Sends a new offer with iceRestart:true via signaling.
   * Resets after a successful answer is received.
   */
  async attemptICERestart() {
    if (!this.pc || !this.signaling || this.iceRestartInProgress) return;
    const maxRestarts = this.config.maxIceRestarts ?? 3;
    if (this.iceRestartAttempts >= maxRestarts) {
      this.emit(
        "error",
        new AftertalkError("webrtc_ice_failed", {
          message: `ICE failed after ${maxRestarts} restart attempts`
        })
      );
      return;
    }
    this.iceRestartInProgress = true;
    this.iceRestartAttempts++;
    this.emit("ice-restarting", this.iceRestartAttempts);
    try {
      this.pc.restartIce();
      const offer = await this.pc.createOffer({ iceRestart: true });
      await this.pc.setLocalDescription(offer);
      this.signaling.send({ type: "offer", sdp: offer.sdp ?? "" });
    } catch (err) {
      this.iceRestartInProgress = false;
      this.emit(
        "error",
        new AftertalkError("webrtc_ice_failed", {
          message: "ICE restart failed",
          details: err
        })
      );
    }
  }
  clearICEDisconnectedTimer() {
    clearTimeout(this.iceDisconnectedTimer);
    this.iceDisconnectedTimer = void 0;
  }
  emit(event, ...args) {
    const set = this.listeners[event];
    if (set) {
      for (const listener of set) {
        try {
          listener(...args);
        } catch {
        }
      }
    }
  }
};

// src/client.ts
var AftertalkClient = class {
  constructor(config) {
    this.clientConfig = config;
    this.http = new HttpClient({
      baseUrl: config.baseUrl,
      apiKey: config.apiKey,
      timeout: config.timeout,
      fetch: config.fetch
    });
    this.sessions = new SessionsAPI(this.http);
    this.transcriptions = new TranscriptionsAPI(this.http);
    this.minutes = new MinutesAPI(this.http);
    this.config = new ConfigAPI(this.http);
    this.rooms = new RoomsAPI(this.http);
    this.poller = new MinutesPoller(this.minutes);
  }
  /**
   * Creates a WebRTCConnection pre-configured with the server's ICE servers.
   * Optionally override ICE servers or signaling URL via webrtcConfig.
   */
  createWebRTCConnection(webrtcConfig) {
    return new WebRTCConnection(webrtcConfig);
  }
  /**
   * High-level helper: acquires ICE servers from server, then connects.
   * Returns the connection ready for use.
   */
  async connectWebRTC(options) {
    const { sessionId, token, webrtcConfig = {} } = options;
    let iceServers;
    if (webrtcConfig.iceServers) {
      iceServers = webrtcConfig.iceServers;
    } else {
      const rtcCfg = await this.config.getRTCConfig();
      iceServers = rtcCfg.iceServers;
    }
    const signalingUrl = webrtcConfig.signalingUrl ?? this.deriveSignalingUrl();
    const conn = new WebRTCConnection(webrtcConfig);
    await conn.connect({ sessionId, token, signalingUrl, iceServers });
    return conn;
  }
  /**
   * Waits for minutes to be ready with exponential backoff polling.
   */
  waitForMinutes(sessionId, options) {
    return this.poller.waitForReady(sessionId, options);
  }
  /**
   * Watches for minute updates continuously, calling onUpdate on each new version.
   */
  watchMinutes(sessionId, onUpdate, options) {
    return this.poller.watch(sessionId, onUpdate, options);
  }
  deriveSignalingUrl() {
    const base = this.clientConfig.baseUrl.replace(/\/$/, "");
    return base.replace(/^http/, "ws") + "/signaling";
  }
};

export { AftertalkClient, AftertalkError, AftertalkClient as AfterthalkClient, AudioManager, ConfigAPI, MinutesAPI, MinutesPoller, RoomsAPI, SessionsAPI, SignalingClient, TranscriptionsAPI, WebRTCConnection };
//# sourceMappingURL=index.mjs.map
//# sourceMappingURL=index.mjs.map