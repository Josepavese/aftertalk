import { AftertalkError } from '../errors.js';
import type { MinutesAPI } from '../api/minutes.js';
import type { Minutes, PollerOptions } from '../types.js';

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export class MinutesPoller {
  constructor(private readonly api: MinutesAPI) {}

  /**
   * Polls until minutes reach `ready` or `delivered` status,
   * using exponential backoff between attempts.
   */
  async waitForReady(sessionId: string, options: PollerOptions = {}): Promise<Minutes> {
    const {
      timeout = 120_000,
      minInterval = 2_000,
      maxInterval = 30_000,
      backoffFactor = 1.5,
    } = options;

    const deadline = Date.now() + timeout;
    let interval = minInterval;

    while (Date.now() < deadline) {
      const minutes = await this.api.getBySession(sessionId);

      if (minutes.status === 'ready' || minutes.status === 'delivered') {
        return minutes;
      }

      if (minutes.status === 'error') {
        throw new AftertalkError('minutes_generation_failed', {
          message: 'Minutes generation failed on server',
          details: minutes,
        });
      }

      const remaining = deadline - Date.now();
      if (remaining <= 0) break;

      await sleep(Math.min(interval, remaining));
      interval = Math.min(interval * backoffFactor, maxInterval);
    }

    throw new AftertalkError('minutes_polling_timeout', {
      message: `Minutes not ready after ${timeout}ms`,
      details: { sessionId, timeout },
    });
  }

  /**
   * Polls with a callback fired each time a new version is detected.
   * Resolves when the session is completed or on timeout.
   */
  async watch(
    sessionId: string,
    onUpdate: (minutes: Minutes) => void,
    options: PollerOptions = {},
  ): Promise<void> {
    const {
      timeout = 300_000,
      minInterval = 5_000,
      maxInterval = 30_000,
      backoffFactor = 1.5,
    } = options;

    const deadline = Date.now() + timeout;
    let interval = minInterval;
    let lastVersion = -1;

    while (Date.now() < deadline) {
      try {
        const minutes = await this.api.getBySession(sessionId);

        if (minutes.version !== lastVersion) {
          lastVersion = minutes.version;
          onUpdate(minutes);
        }

        if (minutes.status === 'delivered') return;
        if (minutes.status === 'error') {
          throw new AftertalkError('minutes_generation_failed', { details: minutes });
        }
      } catch (err) {
        if (err instanceof AftertalkError && err.code === 'not_found') {
          // minutes not yet created, keep polling
        } else {
          throw err;
        }
      }

      const remaining = deadline - Date.now();
      if (remaining <= 0) break;

      await sleep(Math.min(interval, remaining));
      interval = Math.min(interval * backoffFactor, maxInterval);
    }
  }
}
