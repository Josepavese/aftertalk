import { describe, expect, it, vi } from 'vitest';
import type { MinutesAPI } from '../api/minutes.js';
import { AftertalkError } from '../errors.js';
import type { Minutes } from '../types.js';
import { MinutesPoller } from './minutes-poller.js';

function makeMinutes(overrides: Partial<Minutes> = {}): Minutes {
  return {
    id: 'm1',
    sessionId: 's1',
    templateId: 'therapy',
    status: 'pending',
    summary: { overview: '', phases: [] },
    sections: {},
    citations: [],
    provider: 'openai',
    version: 1,
    generatedAt: new Date().toISOString(),
    ...overrides,
  };
}

describe('MinutesPoller', () => {
  it('resolves immediately when status is ready', async () => {
    const api = { getBySession: vi.fn().mockResolvedValue(makeMinutes({ status: 'ready' })) } as unknown as MinutesAPI;
    const poller = new MinutesPoller(api);

    const result = await poller.waitForReady('s1', { minInterval: 10 });
    expect(result.status).toBe('ready');
    expect(api.getBySession).toHaveBeenCalledTimes(1);
  });

  it('resolves when status becomes ready on second poll', async () => {
    const api = {
      getBySession: vi.fn()
        .mockResolvedValueOnce(makeMinutes({ status: 'pending' }))
        .mockResolvedValueOnce(makeMinutes({ status: 'ready' })),
    } as unknown as MinutesAPI;
    const poller = new MinutesPoller(api);

    const result = await poller.waitForReady('s1', { minInterval: 1, maxInterval: 5 });
    expect(result.status).toBe('ready');
    expect(api.getBySession).toHaveBeenCalledTimes(2);
  });

  it('throws minutes_generation_failed on error status', async () => {
    const api = {
      getBySession: vi.fn().mockResolvedValue(makeMinutes({ status: 'error' })),
    } as unknown as MinutesAPI;
    const poller = new MinutesPoller(api);

    await expect(poller.waitForReady('s1', { minInterval: 10 })).rejects.toMatchObject({
      code: 'minutes_generation_failed',
    });
  });

  it('throws minutes_polling_timeout after deadline', async () => {
    const api = {
      getBySession: vi.fn().mockResolvedValue(makeMinutes({ status: 'pending' })),
    } as unknown as MinutesAPI;
    const poller = new MinutesPoller(api);

    await expect(
      poller.waitForReady('s1', { timeout: 50, minInterval: 10, maxInterval: 20 }),
    ).rejects.toMatchObject({
      code: 'minutes_polling_timeout',
    });
  });

  it('watch calls onUpdate on new versions', async () => {
    const v1 = makeMinutes({ status: 'ready', version: 1 });
    const v2 = makeMinutes({ status: 'delivered', version: 2 });

    const api = {
      getBySession: vi.fn()
        .mockResolvedValueOnce(v1)
        .mockResolvedValueOnce(v2),
    } as unknown as MinutesAPI;

    const poller = new MinutesPoller(api);
    const updates: Minutes[] = [];
    await poller.watch('s1', (m) => updates.push(m), { minInterval: 1, maxInterval: 5 });

    expect(updates).toHaveLength(2);
    expect(updates[0].version).toBe(1);
    expect(updates[1].version).toBe(2);
  });

  it('watch skips not_found errors gracefully', async () => {
    const api = {
      getBySession: vi.fn()
        .mockRejectedValueOnce(new AftertalkError('not_found'))
        .mockResolvedValueOnce(makeMinutes({ status: 'delivered', version: 1 })),
    } as unknown as MinutesAPI;

    const poller = new MinutesPoller(api);
    const updates: Minutes[] = [];
    await poller.watch('s1', (m) => updates.push(m), { minInterval: 1, maxInterval: 5 });

    expect(updates).toHaveLength(1);
  });
});
