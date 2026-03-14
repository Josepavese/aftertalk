// Main client
export { AftertalkClient } from './client.js';

// API classes (for direct use if needed)
export { ConfigAPI } from './api/config.js';
export { MinutesAPI } from './api/minutes.js';
export { SessionsAPI } from './api/sessions.js';
export { TranscriptionsAPI } from './api/transcriptions.js';

// WebRTC
export { AudioManager } from './webrtc/audio.js';
export { WebRTCConnection } from './webrtc/connection.js';
export { SignalingClient } from './webrtc/signaling.js';

// Realtime
export { MinutesPoller } from './realtime/minutes-poller.js';

// Errors
export { AftertalkError } from './errors.js';
export type { AftertalkErrorCode } from './errors.js';

// Types
export type {
  AftertalkClientConfig,
  Citation,
  CreateSessionRequest,
  CreateSessionResponse,
  ICEServer,
  Minutes,
  MinutesStatus,
  MinutesVersion,
  Participant,
  ParticipantInput,
  PaginatedResponse,
  PollerOptions,
  RTCConfig,
  RoleConfig,
  SectionConfig,
  SectionType,
  ServerConfig,
  Session,
  SessionFilters,
  SessionStatus,
  Template,
  Transcription,
  TranscriptionFilters,
  TranscriptionStatus,
  UpdateMinutesRequest,
  WebRTCConfig,
} from './types.js';
