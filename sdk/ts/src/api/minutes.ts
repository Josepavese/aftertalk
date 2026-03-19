import type { HttpClient } from '../http.js';
import type { Minutes, MinutesVersion, UpdateMinutesRequest } from '../types.js';

export class MinutesAPI {
  constructor(private readonly http: HttpClient) {}

  /** GET /v1/minutes?session_id={sessionId} */
  async getBySession(sessionId: string): Promise<Minutes> {
    return this.http.get<Minutes>(`/v1/minutes?session_id=${encodeURIComponent(sessionId)}`);
  }

  /** GET /v1/minutes/{minutesId} */
  async get(minutesId: string): Promise<Minutes> {
    return this.http.get<Minutes>(`/v1/minutes/${minutesId}`);
  }

  /** PUT /v1/minutes/{minutesId}  — header X-User-Id to track the editor */
  async update(
    minutesId: string,
    request: UpdateMinutesRequest,
    userId?: string,
  ): Promise<Minutes> {
    const headers: Record<string, string> = userId ? { 'X-User-Id': userId } : {};
    return this.http.put<Minutes>(`/v1/minutes/${minutesId}`, request, { headers });
  }

  /** GET /v1/minutes/{minutesId}/versions */
  async getVersions(minutesId: string): Promise<MinutesVersion[]> {
    return this.http.get<MinutesVersion[]>(`/v1/minutes/${minutesId}/versions`);
  }

  /** DELETE /v1/minutes/{minutesId} */
  async delete(minutesId: string): Promise<void> {
    return this.http.delete<void>(`/v1/minutes/${minutesId}`);
  }
}
