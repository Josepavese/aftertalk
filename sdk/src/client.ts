import { ConfigAPI } from './api/config.js';
import { MinutesAPI } from './api/minutes.js';
import { SessionsAPI } from './api/sessions.js';
import { TranscriptionsAPI } from './api/transcriptions.js';
import { HttpClient } from './http.js';
import { MinutesPoller } from './realtime/minutes-poller.js';
import type { AfterthalkClientConfig, ICEServer, Minutes, PollerOptions, WebRTCConfig } from './types.js';
import { WebRTCConnection } from './webrtc/connection.js';

export class AfterthalkClient {
  readonly sessions: SessionsAPI;
  readonly transcriptions: TranscriptionsAPI;
  readonly minutes: MinutesAPI;
  readonly config: ConfigAPI;

  private readonly http: HttpClient;
  private readonly clientConfig: AfterthalkClientConfig;
  private readonly poller: MinutesPoller;

  constructor(config: AfterthalkClientConfig) {
    this.clientConfig = config;
    this.http = new HttpClient({
      baseUrl: config.baseUrl,
      apiKey: config.apiKey,
      timeout: config.timeout,
      fetch: config.fetch,
    });

    this.sessions = new SessionsAPI(this.http);
    this.transcriptions = new TranscriptionsAPI(this.http);
    this.minutes = new MinutesAPI(this.http);
    this.config = new ConfigAPI(this.http);
    this.poller = new MinutesPoller(this.minutes);
  }

  /**
   * Creates a WebRTCConnection pre-configured with the server's ICE servers.
   * Optionally override ICE servers or signaling URL via webrtcConfig.
   */
  createWebRTCConnection(webrtcConfig?: WebRTCConfig): WebRTCConnection {
    return new WebRTCConnection(webrtcConfig);
  }

  /**
   * High-level helper: acquires ICE servers from server, then connects.
   * Returns the connection ready for use.
   */
  async connectWebRTC(options: {
    sessionId: string;
    token: string;
    webrtcConfig?: WebRTCConfig;
  }): Promise<WebRTCConnection> {
    const { sessionId, token, webrtcConfig = {} } = options;

    let iceServers: ICEServer[];
    if (webrtcConfig.iceServers) {
      iceServers = webrtcConfig.iceServers;
    } else {
      const rtcCfg = await this.config.getRTCConfig();
      iceServers = rtcCfg.iceServers;
    }

    const signalingUrl =
      webrtcConfig.signalingUrl ?? this.deriveSignalingUrl();

    const conn = new WebRTCConnection(webrtcConfig);
    await conn.connect({ sessionId, token, signalingUrl, iceServers });
    return conn;
  }

  /**
   * Waits for minutes to be ready with exponential backoff polling.
   */
  waitForMinutes(sessionId: string, options?: PollerOptions): Promise<Minutes> {
    return this.poller.waitForReady(sessionId, options);
  }

  /**
   * Watches for minute updates continuously, calling onUpdate on each new version.
   */
  watchMinutes(
    sessionId: string,
    onUpdate: (minutes: Minutes) => void,
    options?: PollerOptions,
  ): Promise<void> {
    return this.poller.watch(sessionId, onUpdate, options);
  }

  private deriveSignalingUrl(): string {
    const base = this.clientConfig.baseUrl.replace(/\/$/, '');
    // Convert http(s) → ws(s)
    return base.replace(/^http/, 'ws') + '/signaling';
  }
}
