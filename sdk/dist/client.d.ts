import { EventEmitter } from 'events';
import { SDKConfig, Session, Participant } from './types.js';
export interface AftertalkClientOptions {
    config?: SDKConfig;
    configPath?: string;
}
export declare class AftertalkClient extends EventEmitter {
    private config;
    private logger;
    private currentSession;
    constructor(options?: AftertalkClientOptions);
    connect(): Promise<void>;
    disconnect(): void;
    createSession(title: string): Promise<Session>;
    joinSession(sessionId: string, name: string, role?: 'host' | 'speaker' | 'listener'): Promise<Participant>;
    leaveSession(sessionId: string): Promise<void>;
    getSession(): Session | null;
}
//# sourceMappingURL=client.d.ts.map