import { IHTTPClient, ILogger } from '../middleware/interfaces.js';
export declare class MockHTTPProvider implements IHTTPClient {
    private logger;
    constructor(logger: ILogger);
    get<T>(path: string): Promise<T>;
    post<T>(path: string, data: Record<string, unknown>): Promise<T>;
}
//# sourceMappingURL=http-provider.d.ts.map