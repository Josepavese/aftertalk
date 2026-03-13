import { describe, expect, it, vi } from 'vitest';
import { AftertalkError } from './errors.js';
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
    const fetchMock = makeFetch(200, { sessionId: 'abc' });
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
    expect(headers['X-API-Key']).toBe('secret');
    expect(init.body).toContain('"participantCount":2');
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
