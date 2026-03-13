import { describe, expect, it } from 'vitest';
import { AftertalkError } from './errors.js';

describe('AftertalkError', () => {
  it('sets code and name', () => {
    const err = new AftertalkError('not_found');
    expect(err.code).toBe('not_found');
    expect(err.name).toBe('AftertalkError');
    expect(err.message).toBe('not_found');
  });

  it('accepts custom message', () => {
    const err = new AftertalkError('unauthorized', { message: 'Invalid API key' });
    expect(err.message).toBe('Invalid API key');
    expect(err.status).toBeUndefined();
  });

  it('fromHttpStatus 400', () => {
    const err = AftertalkError.fromHttpStatus(400, { error: 'invalid body' });
    expect(err.code).toBe('bad_request');
    expect(err.status).toBe(400);
    expect(err.message).toBe('invalid body');
  });

  it('fromHttpStatus 401', () => {
    const err = AftertalkError.fromHttpStatus(401);
    expect(err.code).toBe('unauthorized');
  });

  it('fromHttpStatus 404', () => {
    const err = AftertalkError.fromHttpStatus(404, { message: 'session not found' });
    expect(err.code).toBe('not_found');
    expect(err.message).toBe('session not found');
  });

  it('fromHttpStatus 429', () => {
    const err = AftertalkError.fromHttpStatus(429);
    expect(err.code).toBe('rate_limited');
  });

  it('fromHttpStatus 500', () => {
    const err = AftertalkError.fromHttpStatus(500);
    expect(err.code).toBe('server_error');
    expect(err.status).toBe(500);
  });

  it('fromHttpStatus unknown 2xx range falls back to unknown', () => {
    const err = AftertalkError.fromHttpStatus(418);
    expect(err.code).toBe('unknown');
    expect(err.status).toBe(418);
  });
});
