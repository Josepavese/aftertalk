export type AftertalkErrorCode =
  | 'network_error'
  | 'timeout'
  | 'unauthorized'
  | 'forbidden'
  | 'not_found'
  | 'bad_request'
  | 'conflict'
  | 'rate_limited'
  | 'server_error'
  | 'session_not_found'
  | 'session_active'
  | 'minutes_generation_failed'
  | 'minutes_polling_timeout'
  | 'webrtc_connection_failed'
  | 'webrtc_ice_failed'
  | 'signaling_disconnected'
  | 'signaling_reconnect_failed'
  | 'audio_permission_denied'
  | 'audio_device_not_found'
  | 'unknown';

export class AftertalkError extends Error {
  readonly code: AftertalkErrorCode;
  readonly status?: number;
  readonly details?: unknown;

  constructor(code: AftertalkErrorCode, options?: { message?: string; status?: number; details?: unknown }) {
    super(options?.message ?? code);
    this.name = 'AftertalkError';
    this.code = code;
    this.status = options?.status;
    this.details = options?.details;
  }

  static fromHttpStatus(status: number, body?: unknown): AftertalkError {
    const details = body;
    const message = extractMessage(body);

    switch (status) {
      case 400:
        return new AftertalkError('bad_request', { status, message, details });
      case 401:
        return new AftertalkError('unauthorized', { status, message, details });
      case 403:
        return new AftertalkError('forbidden', { status, message, details });
      case 404:
        return new AftertalkError('not_found', { status, message, details });
      case 409:
        return new AftertalkError('conflict', { status, message, details });
      case 429:
        return new AftertalkError('rate_limited', { status, message, details });
      default:
        if (status >= 500) {
          return new AftertalkError('server_error', { status, message, details });
        }
        return new AftertalkError('unknown', { status, message, details });
    }
  }
}

function extractMessage(body: unknown): string | undefined {
  if (!body || typeof body !== 'object') return undefined;
  const b = body as Record<string, unknown>;
  return typeof b['error'] === 'string'
    ? b['error']
    : typeof b['message'] === 'string'
      ? b['message']
      : undefined;
}
