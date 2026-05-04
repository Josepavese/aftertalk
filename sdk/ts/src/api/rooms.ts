import type { HttpClient } from '../http.js';

export interface JoinRoomRequest {
  code:        string;
  name:        string;
  role:        string;
  templateId?: string;
  sttProfile?: string;
  llmProfile?: string;
}

export interface JoinRoomResponse {
  sessionId: string;
  token:     string;
}

export class RoomsAPI {
  constructor(private readonly http: HttpClient) {}

  /**
   * Join or create a room session by code.
   * Creates the session the first time; subsequent participants get their own token.
   * Role is exclusive: two participants cannot share the same role.
   */
  async join(request: JoinRoomRequest): Promise<JoinRoomResponse> {
    const raw = await this.http.post<{ sessionId: string; token: string }>(
      '/v1/rooms/join',
      {
        code:       request.code,
        name:       request.name,
        role:       request.role,
        templateId: request.templateId,
        sttProfile: request.sttProfile,
        llmProfile: request.llmProfile,
      },
    );
    return { sessionId: raw.sessionId, token: raw.token };
  }
}
