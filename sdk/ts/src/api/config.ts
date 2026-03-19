import type { HttpClient } from '../http.js';
import type { ICEServer, RTCConfig, ServerConfig, Template } from '../types.js';

export class ConfigAPI {
  constructor(private readonly http: HttpClient) {}

  /** Public endpoint — no API key required. Returns templates, default_template_id, and provider profiles. */
  async getConfig(): Promise<ServerConfig> {
    const raw = await this.http.get<{
      templates: Template[];
      default_template_id: string;
      stt_profiles?: string[];
      llm_profiles?: string[];
      default_stt_profile?: string;
      default_llm_profile?: string;
    }>('/v1/config');
    return {
      templates:          raw.templates,
      defaultTemplateId:  raw.default_template_id,
      sttProfiles:        raw.stt_profiles,
      llmProfiles:        raw.llm_profiles,
      sttDefaultProfile:  raw.default_stt_profile,
      llmDefaultProfile:  raw.default_llm_profile,
    };
  }

  /** Returns ICE server list for WebRTC, optionally with HMAC credentials. */
  async getRTCConfig(): Promise<RTCConfig> {
    const raw = await this.http.get<{ ice_servers: ICEServer[]; ttl?: number }>('/v1/rtc-config');
    return {
      iceServers: raw.ice_servers ?? [],
      ttl:        raw.ttl,
    };
  }
}
