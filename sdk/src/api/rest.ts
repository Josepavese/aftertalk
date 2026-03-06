import axios, { AxiosInstance } from 'axios';
import { Session, Participant, Transcription, Minutes, SDKConfig } from '../types.js';

export class RESTClient {
  private client: AxiosInstance;

  constructor(config: SDKConfig) {
    this.client = axios.create({
      baseURL: config.sdk.api.baseUrl + config.sdk.api.apiPrefix,
      timeout: config.sdk.api.timeout
    });
  }

  setAuthToken(token: string): void {
    this.client.defaults.headers.common['Authorization'] = `Bearer ${token}`;
  }

  async createSession(data: { title: string }): Promise<Session> {
    const res = await this.client.post<Session>('/sessions', data);
    return res.data;
  }

  async getSession(sessionId: string): Promise<Session> {
    const res = await this.client.get<Session>(`/sessions/${sessionId}`);
    return res.data;
  }

  async joinSession(sessionId: string, data: { name: string; role: string }): Promise<Participant> {
    const res = await this.client.post<Participant>(`/sessions/${sessionId}/participants`, data);
    return res.data;
  }

  async getTranscriptions(sessionId: string): Promise<Transcription[]> {
    const res = await this.client.get<Transcription[]>(`/sessions/${sessionId}/transcriptions`);
    return res.data;
  }

  async requestMinutes(sessionId: string): Promise<Minutes> {
    const res = await this.client.post<Minutes>(`/sessions/${sessionId}/minutes/generate`);
    return res.data;
  }
}
