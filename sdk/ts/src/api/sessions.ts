import type { HttpClient } from '../http.js';
import type {
  CreateSessionRequest,
  CreateSessionResponse,
  PaginatedResponse,
  Session,
  SessionFilters,
} from '../types.js';

type WireSession = Omit<Session, 'sessionId'> & {
  id?: string;
  sessionId?: string;
};

export class SessionsAPI {
  constructor(private readonly http: HttpClient) {}

  async create(request: CreateSessionRequest): Promise<CreateSessionResponse> {
    return this.http.post<CreateSessionResponse>('/v1/sessions', request);
  }

  async get(sessionId: string): Promise<Session> {
    const raw = await this.http.get<WireSession>(`/v1/sessions/${sessionId}`);
    return normalizeSession(raw);
  }

  async getStatus(sessionId: string): Promise<Pick<Session, 'sessionId' | 'status'>> {
    const raw = await this.http.get<{ id?: string; sessionId?: string; status: Session['status'] }>(
      `/v1/sessions/${sessionId}/status`,
    );
    return { sessionId: raw.sessionId ?? raw.id ?? sessionId, status: raw.status };
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
    const raw = await this.http.get<{
      sessions?: WireSession[];
      items?: WireSession[];
      total: number;
      limit: number;
      offset: number;
    }>(`/v1/sessions${qs ? `?${qs}` : ''}`);
    return {
      items: (raw.sessions ?? raw.items ?? []).map(normalizeSession),
      total: raw.total,
      limit: raw.limit,
      offset: raw.offset,
    };
  }

  async delete(sessionId: string): Promise<void> {
    return this.http.delete<void>(`/v1/sessions/${sessionId}`);
  }
}

function normalizeSession(raw: WireSession): Session {
  const { id, sessionId, ...rest } = raw;
  return {
    ...rest,
    sessionId: sessionId ?? id ?? '',
  };
}
