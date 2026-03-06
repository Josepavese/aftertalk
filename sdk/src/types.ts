export interface SDKConfig {
  sdk: {
    version: string;
    environment: string;
    api: APIConfig;
    webrtc: WebRTCConfig;
    signaling: SignalingConfig;
    logging: LoggingConfig;
    audio: AudioConfig;
    session: SessionConfig;
  };
}

export interface APIConfig {
  baseUrl: string;
  apiPrefix: string;
  timeout: number;
}

export interface WebRTCConfig {
  iceServers: ICEServer[];
  audio: AudioConstraints;
}

export interface ICEServer {
  urls: string | string[];
  username?: string;
  credential?: string;
}

export interface AudioConstraints {
  echoCancellation: boolean;
  noiseSuppression: boolean;
  autoGainControl: boolean;
}

export interface SignalingConfig {
  wsUrl: string;
  reconnect: ReconnectConfig;
  pingInterval: number;
}

export interface ReconnectConfig {
  enabled: boolean;
  maxAttempts: number;
  baseDelay: number;
}

export interface LoggingConfig {
  level: 'debug' | 'info' | 'warn' | 'error';
  structured: boolean;
}

export interface AudioConfig {
  sampleRate: number;
  channels: number;
  bufferSize: number;
}

export interface SessionConfig {
  defaultDuration: number;
  maxParticipants: number;
}

export interface Session {
  id: string;
  title: string;
  status: SessionStatus;
  participants: Participant[];
}

export type SessionStatus = 'pending' | 'active' | 'completed' | 'cancelled';

export interface Participant {
  id: string;
  sessionId?: string;
  name: string;
  role: ParticipantRole;
}

export type ParticipantRole = 'host' | 'speaker' | 'listener';

export interface Transcription {
  id: string;
  sessionId: string;
  text: string;
  confidence: number;
  timestamp: Date;
}

export interface Minutes {
  id: string;
  sessionId: string;
  title: string;
  summary: string;
  topics: string[];
  actionItems: ActionItem[];
}

export interface ActionItem {
  id: string;
  description: string;
  completed: boolean;
}

export enum LogLevel {
  DEBUG = 0,
  INFO = 1,
  WARN = 2,
  ERROR = 3
}

export interface ILogger {
  debug(message: string, meta?: Record<string, unknown>): void;
  info(message: string, meta?: Record<string, unknown>): void;
  warn(message: string, meta?: Record<string, unknown>): void;
  error(message: string, meta?: Record<string, unknown>): void;
}
