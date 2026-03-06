export interface TestConfig {
  server: ServerConfig;
  webrtc: WebRTCConfig;
  audio: AudioConfig;
  scenarios: ScenarioConfig[];
}

export interface ServerConfig {
  host: string;
  port: number;
  signalingPath: string;
  healthPath: string;
  timeout: number;
}

export interface WebRTCConfig {
  iceServers: ICEServerConfig[];
  audioSettings: AudioSettingsConfig;
}

export interface ICEServerConfig {
  urls: string[];
  type: 'stun' | 'turn';
  username?: string;
  credential?: string;
}

export interface AudioSettingsConfig {
  codec: string;
  sampleRate: number;
  channels: number;
  bitrate: number;
}

export interface AudioConfig {
  samplePath: string;
  samples: AudioSampleConfig[];
}

export interface AudioSampleConfig {
  name: string;
  filename: string;
  durationMs: number;
}

export interface ScenarioConfig {
  name: string;
  enabled: boolean;
  description: string;
}

export interface TestResult {
  name: string;
  status: 'passed' | 'failed' | 'skipped';
  durationMs: number;
  error?: string;
}
