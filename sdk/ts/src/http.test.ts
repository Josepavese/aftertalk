import { describe, expect, it, vi } from 'vitest';
import { HttpClient } from './http.js';

function makeFetch(status: number, body: unknown, contentType = 'application/json') {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    headers: { get: (h: string) => (h === 'content-type' ? contentType : null) },
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(String(body)),
  });
}

describe('HttpClient', () => {
  it('GET 200 returns parsed body', async () => {
    const fetchMock = makeFetch(200, { session_id: 'abc' });
    const client = new HttpClient({ baseUrl: 'http://localhost:8080', fetch: fetchMock as typeof fetch });

    const result = await client.get<{ sessionId: string }>('/v1/sessions/abc');
    expect(result.sessionId).toBe('abc');
    expect(fetchMock).toHaveBeenCalledWith(
      'http://localhost:8080/v1/sessions/abc',
      expect.objectContaining({ method: 'GET' }),
    );
  });

  it('POST sends JSON body with API key header', async () => {
    const fetchMock = makeFetch(201, { sessionId: 'new-123' });
    const client = new HttpClient({ baseUrl: 'http://localhost', apiKey: 'secret', fetch: fetchMock as typeof fetch });

    await client.post('/v1/sessions', { participantCount: 2 });

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = init.headers as Record<string, string>;
    expect(headers['Authorization']).toBe('Bearer secret');
    expect(init.body).toContain('"participant_count":2');
  });

  it('preserves dynamic minutes section keys while transforming known fields', async () => {
    const fetchMock = makeFetch(200, {
      data: {
        session_id: 's1',
        summary: { overview: 'ok', phases: [{ start_ms: 0, end_ms: 1000 }] },
        sections: { next_steps: [{ due_at_ms: 1000 }] },
        citations: [{ timestamp_ms: 10, text: 'quote' }],
      },
    });
    const client = new HttpClient({ baseUrl: 'http://localhost', fetch: fetchMock as typeof fetch });

    await client.put('/v1/minutes/m1', {
      summary: { phases: [{ startMs: 0, endMs: 1000 }] },
      sections: { next_steps: [{ dueAtMs: 1000 }] },
      citations: [{ timestampMs: 10, text: 'quote' }],
    });
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.body).toContain('"start_ms":0');
    expect(init.body).toContain('"next_steps"');
    expect(init.body).toContain('"dueAtMs":1000');

    const result = await client.get<{
      sessionId: string;
      summary: { phases: Array<{ startMs: number; endMs: number }> };
      sections: Record<string, unknown>;
      citations: Array<{ timestampMs: number }>;
    }>('/v1/minutes/m1');
    expect(result.sessionId).toBe('s1');
    expect(result.summary.phases[0].startMs).toBe(0);
    expect(Object.keys(result.sections)).toEqual(['next_steps']);
    expect(result.citations[0].timestampMs).toBe(10);
  });

  it('DELETE 204 returns undefined', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
      headers: { get: () => null },
    });
    const client = new HttpClient({ baseUrl: 'http://localhost', fetch: fetchMock as typeof fetch });

    const result = await client.delete('/v1/sessions/abc');
    expect(result).toBeUndefined();
  });

  it('throws AftertalkError on 404', async () => {
    const fetchMock = makeFetch(404, { error: 'not found' });
    const client = new HttpClient({ baseUrl: 'http://localhost', fetch: fetchMock as typeof fetch });

    await expect(client.get('/v1/sessions/missing')).rejects.toMatchObject({
      code: 'not_found',
      status: 404,
    });
  });

  it('throws AftertalkError on 401', async () => {
    const fetchMock = makeFetch(401, { error: 'unauthorized' });
    const client = new HttpClient({ baseUrl: 'http://localhost', fetch: fetchMock as typeof fetch });

    await expect(client.get('/v1/sessions')).rejects.toMatchObject({
      code: 'unauthorized',
    });
  });

  it('throws network_error on fetch failure', async () => {
    const fetchMock = vi.fn().mockRejectedValue(new TypeError('failed to fetch'));
    const client = new HttpClient({ baseUrl: 'http://localhost', fetch: fetchMock as typeof fetch });

    await expect(client.get('/v1/sessions')).rejects.toMatchObject({
      code: 'network_error',
    });
  });

  it('strips trailing slash from baseUrl', async () => {
    const fetchMock = makeFetch(200, {});
    const client = new HttpClient({ baseUrl: 'http://localhost:8080/', fetch: fetchMock as typeof fetch });

    await client.get('/v1/health');
    expect(fetchMock.mock.calls[0][0]).toBe('http://localhost:8080/v1/health');
  });
});
