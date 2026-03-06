import { EventEmitter } from 'events';
export declare class SignalingClient extends EventEmitter {
    private wsUrl;
    private ws;
    constructor(wsUrl: string);
    connect(sessionId: string, participantId: string, token: string): Promise<void>;
    send(message: object): void;
    disconnect(): void;
}
//# sourceMappingURL=signaling.d.ts.map