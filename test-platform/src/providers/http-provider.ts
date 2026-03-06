import { IHTTPClient, ILogger } from '../middleware/interfaces.js';

export class MockHTTPProvider implements IHTTPClient {
  constructor(private logger: ILogger) {}

  async get<T>(path: string): Promise<T> {
    this.logger.debug(`GET ${path}`);
    return {} as T;
  }

  async post<T>(path: string, data: Record<string, unknown>): Promise<T> {
    this.logger.debug(`POST ${path}`, data);
    return {} as T;
  }
}
