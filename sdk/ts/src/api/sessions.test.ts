import { describe, expect, it, vi } from 'vitest';
import type { HttpClient } from '../http.js';
import { SessionsAPI } from './sessions.js';

function makeHttp(result: unknown = {}) {
  return {
    get: vi.fn().mockResolvedValue(result),
    post: vi.fn().mockResolvedValue(result),
    put: vi.fn().mockResolvedValue(result),
    delete: vi.fn().mockResolvedValue(undefined),
  } as unknown as HttpClient;
}

describe('SessionsAPI', () => {
  it('create posts to /v1/sessions', async () => {
    const http = makeHttp({ sessionId: 's1', status: 'active', participants: [] });
    const api = new SessionsAPI(http);

    const req = { participantCount: 2, participants: [{ userId: 'u1', role: 'therapist' }, { userId: 'u2', role: 'patient' }] };
    const res = await api.create(req);

    expect(res.sessionId).toBe('s1');
    expect(http.post).toHaveBeenCalledWith('/v1/sessions', req);
  });

  it('get fetches /v1/sessions/{id}', async () => {
    const http = makeHttp({ id: 's1', status: 'active', participants: [], participantCount: 2, createdAt: 'now', updatedAt: 'now' });
    const api = new SessionsAPI(http);

    const session = await api.get('s1');
    expect(session.sessionId).toBe('s1');
    expect(http.get).toHaveBeenCalledWith('/v1/sessions/s1');
  });

  it('getStatus normalizes id to sessionId', async () => {
    const http = makeHttp({ id: 's1', status: 'active' });
    const api = new SessionsAPI(http);

    const status = await api.getStatus('s1');
    expect(status).toEqual({ sessionId: 's1', status: 'active' });
    expect(http.get).toHaveBeenCalledWith('/v1/sessions/s1/status');
  });

  it('end posts to /v1/sessions/{id}/end', async () => {
    const http = makeHttp();
    const api = new SessionsAPI(http);

    await api.end('s1');
    expect(http.post).toHaveBeenCalledWith('/v1/sessions/s1/end');
  });

  it('delete calls DELETE /v1/sessions/{id}', async () => {
    const http = makeHttp();
    const api = new SessionsAPI(http);

    await api.delete('s1');
    expect(http.delete).toHaveBeenCalledWith('/v1/sessions/s1');
  });

  it('list with filters builds query string', async () => {
    const http = makeHttp({ sessions: [{ id: 's1', status: 'active', participants: [], participantCount: 2, createdAt: 'now', updatedAt: 'now' }], total: 1, limit: 10, offset: 20 });
    const api = new SessionsAPI(http);

    const list = await api.list({ status: 'active', limit: 10, offset: 20 });
    expect(list.items[0].sessionId).toBe('s1');
    expect(http.get).toHaveBeenCalledWith('/v1/sessions?status=active&limit=10&offset=20');
  });

  it('list without filters omits query string', async () => {
    const http = makeHttp({ items: [], total: 0, limit: 10, offset: 0 });
    const api = new SessionsAPI(http);

    await api.list();
    expect(http.get).toHaveBeenCalledWith('/v1/sessions');
  });
});
