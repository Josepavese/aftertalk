import type { HttpClient } from '../http.js';
import type {
  CreateSessionRequest,
  CreateSessionResponse,
  PaginatedResponse,
  Session,
  SessionFilters,
} from '../types.js';

export class SessionsAPI {
  constructor(private readonly http: HttpClient) {}

  async create(request: CreateSessionRequest): Promise<CreateSessionResponse> {
    return this.http.post<CreateSessionResponse>('/v1/sessions', request);
  }

  async get(sessionId: string): Promise<Session> {
    return this.http.get<Session>(`/v1/sessions/${sessionId}`);
  }

  async getStatus(sessionId: string): Promise<Pick<Session, 'sessionId' | 'status' | 'updatedAt'>> {
    return this.http.get(`/v1/sessions/${sessionId}/status`);
  }

  async end(sessionId: string): Promise<void> {
    return this.http.post<void>(`/v1/sessions/${sessionId}/end`);
  }

  async list(filters?: SessionFilters): Promise<PaginatedResponse<Session>> {
    const params = new URLSearchParams();
    if (filters?.status) params.set('status', filters.status);
    if (filters?.limit !== undefined) params.set('limit', String(filters.limit));
    if (filters?.offset !== undefined) params.set('offset', String(filters.offset));

    const qs = params.toString();
    return this.http.get<PaginatedResponse<Session>>(`/v1/sessions${qs ? `?${qs}` : ''}`);
  }

  async delete(sessionId: string): Promise<void> {
    return this.http.delete<void>(`/v1/sessions/${sessionId}`);
  }
}
