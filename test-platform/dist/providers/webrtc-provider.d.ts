import { IWebRTCConnection, ILogger } from '../middleware/interfaces.js';
export declare class MockWebRTCProvider implements IWebRTCConnection {
    private logger;
    private connected;
    constructor(logger: ILogger);
    connect(url: string, _token: string): Promise<void>;
    disconnect(): Promise<void>;
    isConnected(): boolean;
}
//# sourceMappingURL=webrtc-provider.d.ts.map