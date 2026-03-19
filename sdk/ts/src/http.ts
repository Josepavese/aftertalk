import { AftertalkError } from './errors.js';

export interface RequestOptions {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
  signal?: AbortSignal;
}

export class HttpClient {
  private readonly baseUrl: string;
  private readonly apiKey?: string;
  private readonly timeout: number;
  private readonly fetchImpl: typeof fetch;

  constructor(options: {
    baseUrl: string;
    apiKey?: string;
    timeout?: number;
    fetch?: typeof fetch;
  }) {
    this.baseUrl = options.baseUrl.replace(/\/$/, '');
    this.apiKey = options.apiKey;
    this.timeout = options.timeout ?? 30_000;
    this.fetchImpl = options.fetch ?? globalThis.fetch.bind(globalThis);
  }

  async get<T>(path: string, options?: Omit<RequestOptions, 'method' | 'body'>): Promise<T> {
    return this.request<T>(path, { ...options, method: 'GET' });
  }

  async post<T>(path: string, body?: unknown, options?: Omit<RequestOptions, 'method' | 'body'>): Promise<T> {
    return this.request<T>(path, { ...options, method: 'POST', body });
  }

  async put<T>(path: string, body?: unknown, options?: Omit<RequestOptions, 'method' | 'body'>): Promise<T> {
    return this.request<T>(path, { ...options, method: 'PUT', body });
  }

  async delete<T = void>(path: string, options?: Omit<RequestOptions, 'method' | 'body'>): Promise<T> {
    return this.request<T>(path, { ...options, method: 'DELETE' });
  }

  private async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const { method = 'GET', body, headers = {}, signal } = options;

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);
    const combinedSignal = signal
      ? anySignal([signal, controller.signal])
      : controller.signal;

    const reqHeaders: Record<string, string> = {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
      ...headers,
    };
    if (this.apiKey) {
      reqHeaders['X-API-Key'] = this.apiKey;
    }

    let response: Response;
    try {
      response = await this.fetchImpl(`${this.baseUrl}${path}`, {
        method,
        headers: reqHeaders,
        body: body !== undefined ? JSON.stringify(body) : undefined,
        signal: combinedSignal,
      });
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') {
        throw new AftertalkError('timeout', { message: `Request timed out after ${this.timeout}ms` });
      }
      throw new AftertalkError('network_error', {
        message: err instanceof Error ? err.message : 'Network request failed',
        details: err,
      });
    } finally {
      clearTimeout(timeoutId);
    }

    if (response.status === 204) {
      return undefined as T;
    }

    let responseBody: unknown;
    const contentType = response.headers.get('content-type') ?? '';
    if (contentType.includes('application/json')) {
      responseBody = await response.json().catch(() => null);
    } else {
      responseBody = await response.text().catch(() => null);
    }

    if (!response.ok) {
      throw AftertalkError.fromHttpStatus(response.status, responseBody);
    }

    // The server wraps successful responses in { "data": ... } — unwrap transparently.
    if (responseBody !== null && typeof responseBody === 'object' && 'data' in (responseBody as object)) {
      return (responseBody as { data: T }).data;
    }
    return responseBody as T;
  }
}

/** Aborts as soon as any of the given signals fires. */
function anySignal(signals: AbortSignal[]): AbortSignal {
  const controller = new AbortController();
  const onAbort = () => controller.abort();
  for (const signal of signals) {
    if (signal.aborted) {
      controller.abort();
      break;
    }
    signal.addEventListener('abort', onAbort, { once: true });
  }
  // When the combined signal itself aborts, clean up all upstream listeners.
  controller.signal.addEventListener('abort', () => {
    for (const signal of signals) {
      signal.removeEventListener('abort', onAbort);
    }
  }, { once: true });
  return controller.signal;
}
