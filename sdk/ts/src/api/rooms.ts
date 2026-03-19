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
    const raw = await this.http.post<{ session_id: string; token: string }>(
      '/v1/rooms/join',
      {
        code:        request.code,
        name:        request.name,
        role:        request.role,
        template_id: request.templateId,
        stt_profile: request.sttProfile,
        llm_profile: request.llmProfile,
      },
    );
    return { sessionId: raw.session_id, token: raw.token };
  }
}
