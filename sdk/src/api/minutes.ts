import type { HttpClient } from '../http.js';
import type { Minutes, MinutesVersion, UpdateMinutesRequest } from '../types.js';

export class MinutesAPI {
  constructor(private readonly http: HttpClient) {}

  async getBySession(sessionId: string): Promise<Minutes> {
    return this.http.get<Minutes>(`/v1/sessions/${sessionId}/minutes`);
  }

  async update(sessionId: string, request: UpdateMinutesRequest): Promise<Minutes> {
    return this.http.put<Minutes>(`/v1/sessions/${sessionId}/minutes`, request);
  }

  async getVersions(sessionId: string): Promise<MinutesVersion[]> {
    return this.http.get<MinutesVersion[]>(`/v1/sessions/${sessionId}/minutes/versions`);
  }

  async delete(minutesId: string): Promise<void> {
    return this.http.delete<void>(`/v1/minutes/${minutesId}`);
  }
}
