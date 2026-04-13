interface RequestOptions {
    method?: string;
    body?: unknown;
    headers?: Record<string, string>;
    signal?: AbortSignal;
}
declare class HttpClient {
    private readonly baseUrl;
    private readonly apiKey?;
    private readonly timeout;
    private readonly fetchImpl;
    constructor(options: {
        baseUrl: string;
        apiKey?: string;
        timeout?: number;
        fetch?: typeof fetch;
    });
    get<T>(path: string, options?: Omit<RequestOptions, 'method' | 'body'>): Promise<T>;
    post<T>(path: string, body?: unknown, options?: Omit<RequestOptions, 'method' | 'body'>): Promise<T>;
    put<T>(path: string, body?: unknown, options?: Omit<RequestOptions, 'method' | 'body'>): Promise<T>;
    delete<T = void>(path: string, options?: Omit<RequestOptions, 'method' | 'body'>): Promise<T>;
    private request;
}

type SessionStatus = 'active' | 'ended' | 'processing' | 'completed' | 'error';
interface Session {
    sessionId: string;
    status: SessionStatus;
    templateId?: string;
    participantCount: number;
    participants: Participant[];
    createdAt: string;
    endedAt?: string;
    updatedAt: string;
}
interface Participant {
    participantId: string;
    userId: string;
    role: string;
    token: string;
    connectedAt?: string;
    audioStreamId?: string;
}
interface CreateSessionRequest {
    participantCount: number;
    templateId?: string;
    /**
     * Opaque JSON string stored with the session and propagated unchanged to
     * every webhook delivery. Use it to carry your own routing context
     * (e.g. appointment ID, doctor ID) so webhook recipients can associate
     * the minutes without maintaining a separate lookup table.
     *
     * Example: `JSON.stringify({ appointmentId: 'appt_123', doctorId: 'doc_456' })`
     *
     * Aftertalk never inspects or modifies this value.
     * Must be set server-side; never derive it from client input.
     */
    metadata?: string;
    /**
     * Named STT provider profile to use for this session (e.g. "local", "cloud").
     * Must match a profile defined in the server's `stt.profiles` configuration.
     * Falls back to the server default when omitted.
     */
    sttProfile?: string;
    /**
     * Named LLM provider profile to use for this session (e.g. "local", "cloud").
     * Must match a profile defined in the server's `llm.profiles` configuration.
     * Falls back to the server default when omitted.
     */
    llmProfile?: string;
    participants: ParticipantInput[];
}
interface ParticipantInput {
    userId: string;
    role: string;
}
interface CreateSessionResponse {
    sessionId: string;
    status: SessionStatus;
    templateId?: string;
    participants: Participant[];
    createdAt: string;
}
interface SessionFilters {
    status?: SessionStatus;
    limit?: number;
    offset?: number;
}
type TranscriptionStatus = 'pending' | 'processing' | 'ready' | 'error';
interface Transcription {
    id: string;
    sessionId: string;
    role: string;
    text: string;
    status: TranscriptionStatus;
    confidence: number;
    startedAtMs: number;
    endedAtMs: number;
    createdAt: string;
}
interface TranscriptionFilters {
    limit?: number;
    offset?: number;
}
type MinutesStatus = 'pending' | 'ready' | 'delivered' | 'error';
interface Minutes {
    id: string;
    sessionId: string;
    templateId: string;
    status: MinutesStatus;
    summary: MinutesSummary;
    sections: Record<string, unknown>;
    citations: Citation[];
    provider: string;
    version: number;
    generatedAt: string;
}
interface MinutesVersion {
    id: string;
    sessionId: string;
    version: number;
    summary?: MinutesSummary;
    sections: Record<string, unknown>;
    citations: Citation[];
    updatedAt: string;
    updatedBy?: string;
}
interface MinutesSummary {
    overview: string;
    phases: MinutesPhase[];
}
interface MinutesPhase {
    title: string;
    summary: string;
    startMs: number;
    endMs: number;
}
interface Citation {
    text: string;
    role: string;
    timestampMs: number;
}
interface UpdateMinutesRequest {
    summary?: MinutesSummary;
    sections?: Record<string, unknown>;
    citations?: Citation[];
}
interface Template {
    id: string;
    name: string;
    description: string;
    roles: RoleConfig[];
    sections: SectionConfig[];
}
interface RoleConfig {
    key: string;
    label: string;
}
type SectionType = 'string_list' | 'content_items' | 'progress';
interface SectionConfig {
    key: string;
    label: string;
    description: string;
    type: SectionType;
}
interface ServerConfig {
    templates: Template[];
    defaultTemplateId: string;
    /** Available STT provider profile names (e.g. ["local", "cloud"]). Present when ≥1 profile is configured. */
    sttProfiles?: string[];
    /** Default STT profile name. */
    sttDefaultProfile?: string;
    /** Available LLM provider profile names (e.g. ["local", "cloud"]). Present when ≥1 profile is configured. */
    llmProfiles?: string[];
    /** Default LLM profile name. */
    llmDefaultProfile?: string;
}
interface ICEServer {
    urls: string | string[];
    username?: string;
    credential?: string;
}
interface RTCConfig {
    iceServers: ICEServer[];
    ttl?: number;
}
interface PaginatedResponse<T> {
    items: T[];
    total: number;
    limit: number;
    offset: number;
}
interface AftertalkClientConfig {
    /** Base URL of the Aftertalk server, e.g. "http://localhost:8080" */
    baseUrl: string;
    /** API key for authenticated endpoints */
    apiKey?: string;
    /** Request timeout in ms (default: 30000) */
    timeout?: number;
    /** Custom fetch implementation (useful for testing/SSR) */
    fetch?: typeof fetch;
}
interface WebRTCConfig {
    /** Signaling WebSocket URL (defaults to baseUrl/signaling) */
    signalingUrl?: string;
    /** Override ICE servers (defaults to fetching from /v1/rtc-config) */
    iceServers?: ICEServer[];
    /** Max reconnect attempts for signaling WS (default: 5) */
    maxReconnectAttempts?: number;
    /** Audio constraints passed to getUserMedia */
    audioConstraints?: MediaTrackConstraints;
    /**
     * Optional callback to obtain a fresh token on each WS connect attempt.
     * Use this when tokens are short-lived (e.g. JWT with 1h TTL) to prevent
     * reconnects failing with 401.
     */
    tokenProvider?: () => string | Promise<string>;
    /**
     * Fractional jitter applied to the WS reconnect backoff (0–1, default: 0.3).
     * Prevents thundering herd when many clients reconnect simultaneously.
     */
    backoffJitter?: number;
    /**
     * How long (ms) to wait after ICE enters `disconnected` before attempting
     * an ICE restart. The browser may recover on its own during this period.
     * Default: 5000.
     */
    iceDisconnectedGraceMs?: number;
    /**
     * Maximum number of ICE restart attempts before emitting a terminal error.
     * Default: 3.
     */
    maxIceRestarts?: number;
}
interface PollerOptions {
    /** Max wait time in ms (default: 120_000) */
    timeout?: number;
    /** Initial polling interval in ms (default: 2_000) */
    minInterval?: number;
    /** Max polling interval ms (default: 30_000) */
    maxInterval?: number;
    /** Backoff multiplier (default: 1.5) */
    backoffFactor?: number;
}

declare class ConfigAPI {
    private readonly http;
    constructor(http: HttpClient);
    /** Public endpoint — no API key required. Returns templates, default_template_id, and provider profiles. */
    getConfig(): Promise<ServerConfig>;
    /** Returns ICE server list for WebRTC, optionally with HMAC credentials. */
    getRTCConfig(): Promise<RTCConfig>;
}

declare class MinutesAPI {
    private readonly http;
    constructor(http: HttpClient);
    /** GET /v1/minutes?session_id={sessionId} */
    getBySession(sessionId: string): Promise<Minutes>;
    /** GET /v1/minutes/{minutesId} */
    get(minutesId: string): Promise<Minutes>;
    /** PUT /v1/minutes/{minutesId}  — header X-User-Id to track the editor */
    update(minutesId: string, request: UpdateMinutesRequest, userId?: string): Promise<Minutes>;
    /** GET /v1/minutes/{minutesId}/versions */
    getVersions(minutesId: string): Promise<MinutesVersion[]>;
    /** DELETE /v1/minutes/{minutesId} */
    delete(minutesId: string): Promise<void>;
}

interface JoinRoomRequest {
    code: string;
    name: string;
    role: string;
    templateId?: string;
    sttProfile?: string;
    llmProfile?: string;
}
interface JoinRoomResponse {
    sessionId: string;
    token: string;
}
declare class RoomsAPI {
    private readonly http;
    constructor(http: HttpClient);
    /**
     * Join or create a room session by code.
     * Creates the session the first time; subsequent participants get their own token.
     * Role is exclusive: two participants cannot share the same role.
     */
    join(request: JoinRoomRequest): Promise<JoinRoomResponse>;
}

declare class SessionsAPI {
    private readonly http;
    constructor(http: HttpClient);
    create(request: CreateSessionRequest): Promise<CreateSessionResponse>;
    get(sessionId: string): Promise<Session>;
    getStatus(sessionId: string): Promise<Pick<Session, 'sessionId' | 'status' | 'updatedAt'>>;
    end(sessionId: string): Promise<void>;
    list(filters?: SessionFilters): Promise<PaginatedResponse<Session>>;
    delete(sessionId: string): Promise<void>;
}

declare class TranscriptionsAPI {
    private readonly http;
    constructor(http: HttpClient);
    /** GET /v1/transcriptions?session_id={sessionId} */
    listBySession(sessionId: string, filters?: TranscriptionFilters): Promise<PaginatedResponse<Transcription>>;
    /** GET /v1/transcriptions/{transcriptionId} */
    get(transcriptionId: string): Promise<Transcription>;
}

type AftertalkErrorCode = 'network_error' | 'timeout' | 'unauthorized' | 'forbidden' | 'not_found' | 'bad_request' | 'conflict' | 'rate_limited' | 'server_error' | 'session_not_found' | 'session_active' | 'minutes_generation_failed' | 'minutes_polling_timeout' | 'webrtc_connection_failed' | 'webrtc_ice_failed' | 'signaling_disconnected' | 'signaling_reconnect_failed' | 'audio_permission_denied' | 'audio_device_not_found' | 'unknown';
declare class AftertalkError extends Error {
    readonly code: AftertalkErrorCode;
    readonly status?: number;
    readonly details?: unknown;
    constructor(code: AftertalkErrorCode, options?: {
        message?: string;
        status?: number;
        details?: unknown;
    });
    static fromHttpStatus(status: number, body?: unknown): AftertalkError;
}

type ConnectionEventMap = {
    connected: [sessionId: string];
    disconnected: [reason: string];
    'audio-started': [];
    'ice-state-changed': [state: RTCIceConnectionState];
    'ice-restarting': [attempt: number];
    'signaling-reconnecting': [attempt: number];
    error: [err: AftertalkError];
};
type ConnectionListener<K extends keyof ConnectionEventMap> = (...args: ConnectionEventMap[K]) => void;
declare class WebRTCConnection {
    private readonly config;
    private pc?;
    private signaling?;
    private audio;
    private _sessionId?;
    private iceDisconnectedTimer?;
    private iceRestartAttempts;
    private iceRestartInProgress;
    private listeners;
    constructor(config?: WebRTCConfig);
    get sessionId(): string | undefined;
    get muted(): boolean;
    connect(options: {
        sessionId: string;
        token: string;
        signalingUrl: string;
        iceServers: ICEServer[];
    }): Promise<void>;
    setMuted(muted: boolean): void;
    disconnect(): Promise<void>;
    on<K extends keyof ConnectionEventMap>(event: K, listener: ConnectionListener<K>): this;
    off<K extends keyof ConnectionEventMap>(event: K, listener: ConnectionListener<K>): this;
    private setupPCListeners;
    /**
     * Attempts to recover a broken ICE connection via ICE restart (RFC 8445 §9.3.2).
     * Sends a new offer with iceRestart:true via signaling.
     * Resets after a successful answer is received.
     */
    private attemptICERestart;
    private clearICEDisconnectedTimer;
    private emit;
}

declare class AftertalkClient {
    readonly sessions: SessionsAPI;
    readonly transcriptions: TranscriptionsAPI;
    readonly minutes: MinutesAPI;
    readonly config: ConfigAPI;
    readonly rooms: RoomsAPI;
    private readonly http;
    private readonly clientConfig;
    private readonly poller;
    constructor(config: AftertalkClientConfig);
    /**
     * Creates a WebRTCConnection pre-configured with the server's ICE servers.
     * Optionally override ICE servers or signaling URL via webrtcConfig.
     */
    createWebRTCConnection(webrtcConfig?: WebRTCConfig): WebRTCConnection;
    /**
     * High-level helper: acquires ICE servers from server, then connects.
     * Returns the connection ready for use.
     */
    connectWebRTC(options: {
        sessionId: string;
        token: string;
        webrtcConfig?: WebRTCConfig;
    }): Promise<WebRTCConnection>;
    /**
     * Waits for minutes to be ready with exponential backoff polling.
     */
    waitForMinutes(sessionId: string, options?: PollerOptions): Promise<Minutes>;
    /**
     * Watches for minute updates continuously, calling onUpdate on each new version.
     */
    watchMinutes(sessionId: string, onUpdate: (minutes: Minutes) => void, options?: PollerOptions): Promise<void>;
    private deriveSignalingUrl;
}

declare class AudioManager {
    private stream?;
    private _muted;
    get muted(): boolean;
    get active(): boolean;
    acquire(constraints?: MediaTrackConstraints): Promise<MediaStream>;
    setMuted(muted: boolean): void;
    release(): void;
}

type SignalingMessageType = 'offer' | 'answer' | 'ice-candidate' | 'error' | 'ping' | 'pong';
interface SignalingMessage {
    type: SignalingMessageType;
    [key: string]: unknown;
}
interface ICECandidateMessage extends SignalingMessage {
    type: 'ice-candidate';
    candidate: RTCIceCandidateInit;
}
interface AnswerMessage extends SignalingMessage {
    type: 'answer';
    sdp: string;
}
interface SignalingClientOptions {
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
type SignalingEventMap = {
    connected: [];
    disconnected: [reason: string];
    reconnecting: [attempt: number];
    message: [msg: SignalingMessage];
    answer: [msg: AnswerMessage];
    'ice-candidate': [msg: ICECandidateMessage];
    error: [err: AftertalkError];
};
type SignalingListener<K extends keyof SignalingEventMap> = (...args: SignalingEventMap[K]) => void;
declare class SignalingClient {
    private ws?;
    private _connected;
    private _closed;
    private reconnectAttempts;
    private reconnectTimer?;
    private pingTimer?;
    private messageQueue;
    private readonly options;
    private listeners;
    constructor(options: SignalingClientOptions);
    get connected(): boolean;
    connect(): Promise<void>;
    send(message: SignalingMessage): void;
    close(): void;
    on<K extends keyof SignalingEventMap>(event: K, listener: SignalingListener<K>): this;
    off<K extends keyof SignalingEventMap>(event: K, listener: SignalingListener<K>): this;
    private emit;
    private attachListeners;
    private detachListeners;
    private onMessage;
    private onClose;
    private resolveToken;
    private backoffDelay;
    private flushQueue;
    private startPing;
    private stopPing;
}

declare class MinutesPoller {
    private readonly api;
    constructor(api: MinutesAPI);
    /**
     * Polls until minutes reach `ready` or `delivered` status,
     * using exponential backoff between attempts.
     */
    waitForReady(sessionId: string, options?: PollerOptions): Promise<Minutes>;
    /**
     * Polls with a callback fired each time a new version is detected.
     * Resolves when the session is completed or on timeout.
     */
    watch(sessionId: string, onUpdate: (minutes: Minutes) => void, options?: PollerOptions): Promise<void>;
}

export { AftertalkClient, type AftertalkClientConfig, AftertalkError, type AftertalkErrorCode, AftertalkClient as AfterthalkClient, AudioManager, type Citation, ConfigAPI, type CreateSessionRequest, type CreateSessionResponse, type ICEServer, type JoinRoomRequest, type JoinRoomResponse, type Minutes, MinutesAPI, MinutesPoller, type MinutesStatus, type MinutesVersion, type PaginatedResponse, type Participant, type ParticipantInput, type PollerOptions, type RTCConfig, type RoleConfig, RoomsAPI, type SectionConfig, type SectionType, type ServerConfig, type Session, type SessionFilters, type SessionStatus, SessionsAPI, SignalingClient, type Template, type Transcription, type TranscriptionFilters, type TranscriptionStatus, TranscriptionsAPI, type UpdateMinutesRequest, type WebRTCConfig, WebRTCConnection };
