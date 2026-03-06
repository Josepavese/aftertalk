export interface IWebRTCConnection {
    connect(url: string, token: string): Promise<void>;
    disconnect(): Promise<void>;
    isConnected(): boolean;
}
export interface ISignalingClient {
    connect(url: string): Promise<void>;
    disconnect(): Promise<void>;
    sendOffer(sdp: string): Promise<string>;
}
export interface IHTTPClient {
    get<T>(path: string): Promise<T>;
    post<T>(path: string, data: unknown): Promise<T>;
}
export interface ILogger {
    debug(message: string, meta?: Record<string, unknown>): void;
    info(message: string, meta?: Record<string, unknown>): void;
    error(message: string, meta?: Record<string, unknown>): void;
}
export interface ITestReporter {
    report(test: unknown): Promise<void>;
    summary(results: unknown[]): Promise<unknown>;
}
export declare enum ConnectionState {
    DISCONNECTED = "disconnected",
    CONNECTING = "connecting",
    CONNECTED = "connected"
}
//# sourceMappingURL=interfaces.d.ts.map