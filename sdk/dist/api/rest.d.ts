import { Session, Participant, Transcription, Minutes, SDKConfig } from '../types.js';
export declare class RESTClient {
    private client;
    constructor(config: SDKConfig);
    setAuthToken(token: string): void;
    createSession(data: {
        title: string;
    }): Promise<Session>;
    getSession(sessionId: string): Promise<Session>;
    joinSession(sessionId: string, data: {
        name: string;
        role: string;
    }): Promise<Participant>;
    getTranscriptions(sessionId: string): Promise<Transcription[]>;
    requestMinutes(sessionId: string): Promise<Minutes>;
}
//# sourceMappingURL=rest.d.ts.map