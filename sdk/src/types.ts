// ─── Session ─────────────────────────────────────────────────────────────────

export type SessionStatus = 'active' | 'ended' | 'processing' | 'completed' | 'error';

export interface Session {
  sessionId: string;
  status: SessionStatus;
  templateId?: string;
  participantCount: number;
  participants: Participant[];
  createdAt: string;
  endedAt?: string;
  updatedAt: string;
}

export interface Participant {
  participantId: string;
  userId: string;
  role: string;
  token: string;
  connectedAt?: string;
  audioStreamId?: string;
}

export interface CreateSessionRequest {
  participantCount: number;
  templateId?: string;
  participants: ParticipantInput[];
}

export interface ParticipantInput {
  userId: string;
  role: string;
}

export interface CreateSessionResponse {
  sessionId: string;
  status: SessionStatus;
  templateId?: string;
  participants: Participant[];
  createdAt: string;
}

export interface SessionFilters {
  status?: SessionStatus;
  limit?: number;
  offset?: number;
}

// ─── Transcription ────────────────────────────────────────────────────────────

export type TranscriptionStatus = 'pending' | 'processing' | 'ready' | 'error';

export interface Transcription {
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

export interface TranscriptionFilters {
  limit?: number;
  offset?: number;
}

// ─── Minutes ─────────────────────────────────────────────────────────────────

export type MinutesStatus = 'pending' | 'ready' | 'delivered' | 'error';

export interface Minutes {
  id: string;
  sessionId: string;
  templateId: string;
  status: MinutesStatus;
  sections: Record<string, unknown>;
  citations: Citation[];
  provider: string;
  version: number;
  generatedAt: string;
}

export interface MinutesVersion {
  id: string;
  sessionId: string;
  version: number;
  sections: Record<string, unknown>;
  citations: Citation[];
  updatedAt: string;
  updatedBy?: string;
}

export interface Citation {
  text: string;
  role: string;
  timestampMs: number;
}

export interface UpdateMinutesRequest {
  sections?: Record<string, unknown>;
  notes?: string;
}

// ─── Templates ───────────────────────────────────────────────────────────────

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

export type SectionType = 'string_list' | 'content_items' | 'progress';

export interface SectionConfig {
  key: string;
  label: string;
  description: string;
  type: SectionType;
}

// ─── Config ───────────────────────────────────────────────────────────────────

export interface ServerConfig {
  templates: Template[];
  defaultTemplateId: string;
}

// ─── RTC ─────────────────────────────────────────────────────────────────────

export interface ICEServer {
  urls: string | string[];
  username?: string;
  credential?: string;
}

export interface RTCConfig {
  iceServers: ICEServer[];
  ttl?: number;
}

// ─── Pagination ───────────────────────────────────────────────────────────────

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

// ─── SDK Config ───────────────────────────────────────────────────────────────

export interface AfterthalkClientConfig {
  /** Base URL of the Aftertalk server, e.g. "http://localhost:8080" */
  baseUrl: string;
  /** API key for authenticated endpoints */
  apiKey?: string;
  /** Request timeout in ms (default: 30000) */
  timeout?: number;
  /** Custom fetch implementation (useful for testing/SSR) */
  fetch?: typeof fetch;
}

export interface WebRTCConfig {
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

export interface PollerOptions {
  /** Max wait time in ms (default: 120_000) */
  timeout?: number;
  /** Initial polling interval in ms (default: 2_000) */
  minInterval?: number;
  /** Max polling interval ms (default: 30_000) */
  maxInterval?: number;
  /** Backoff multiplier (default: 1.5) */
  backoffFactor?: number;
}
