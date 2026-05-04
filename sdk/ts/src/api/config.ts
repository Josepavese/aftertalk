import type { HttpClient } from '../http.js';
import type { ICEServer, RTCConfig, ServerConfig } from '../types.js';

export class ConfigAPI {
  constructor(private readonly http: HttpClient) {}

  /** Public endpoint — no API key required. Returns templates, default_template_id, and provider profiles. */
  async getConfig(): Promise<ServerConfig> {
    const raw = await this.http.get<ServerConfig>('/v1/config');
    return {
      templates:          raw.templates,
      defaultTemplateId:  raw.defaultTemplateId,
      sttProfiles:        raw.sttProfiles,
      llmProfiles:        raw.llmProfiles,
      sttDefaultProfile:  raw.sttDefaultProfile,
      llmDefaultProfile:  raw.llmDefaultProfile,
    };
  }

  /** Returns ICE server list for WebRTC, optionally with HMAC credentials. */
  async getRTCConfig(): Promise<RTCConfig> {
    const raw = await this.http.get<{ iceServers: ICEServer[]; ttl?: number }>('/v1/rtc-config');
    return {
      iceServers: raw.iceServers ?? [],
      ttl:        raw.ttl,
    };
  }
}
