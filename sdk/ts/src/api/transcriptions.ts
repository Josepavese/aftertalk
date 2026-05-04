import type { HttpClient } from '../http.js';
import type { PaginatedResponse, Transcription, TranscriptionFilters } from '../types.js';

export class TranscriptionsAPI {
  constructor(private readonly http: HttpClient) {}

  /** GET /v1/transcriptions?session_id={sessionId} */
  async listBySession(
    sessionId: string,
    filters?: TranscriptionFilters,
  ): Promise<PaginatedResponse<Transcription>> {
    const params = new URLSearchParams({ session_id: sessionId });
    if (filters?.limit !== undefined) params.set('limit', String(filters.limit));
    if (filters?.offset !== undefined) params.set('offset', String(filters.offset));
    const raw = await this.http.get<{
      transcriptions?: Transcription[];
      items?: Transcription[];
      total: number;
      limit: number;
      offset: number;
    }>(`/v1/transcriptions?${params}`);
    return {
      items: raw.transcriptions ?? raw.items ?? [],
      total: raw.total,
      limit: raw.limit,
      offset: raw.offset,
    };
  }

  /** GET /v1/transcriptions/{transcriptionId} */
  async get(transcriptionId: string): Promise<Transcription> {
    return this.http.get<Transcription>(`/v1/transcriptions/${transcriptionId}`);
  }
}
