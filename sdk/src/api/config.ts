import type { HttpClient } from '../http.js';
import type { RTCConfig, ServerConfig } from '../types.js';

export class ConfigAPI {
  constructor(private readonly http: HttpClient) {}

  /** Public endpoint — no API key required. Returns templates + default_template_id. */
  async getServerConfig(): Promise<ServerConfig> {
    return this.http.get<ServerConfig>('/v1/config');
  }

  /** Returns ICE server list for WebRTC, optionally with HMAC credentials. */
  async getRTCConfig(): Promise<RTCConfig> {
    return this.http.get<RTCConfig>('/v1/rtc-config');
  }
}
