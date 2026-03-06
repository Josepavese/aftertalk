import { EventEmitter } from 'events';
import { ICEServer } from '../types.js';
export declare class WebRTCConnection extends EventEmitter {
    private iceServers;
    private peerConnection;
    constructor(iceServers?: ICEServer[]);
    connect(): Promise<void>;
    createOffer(): Promise<RTCSessionDescriptionInit>;
    handleOffer(offer: RTCSessionDescriptionInit): Promise<RTCSessionDescriptionInit>;
    close(): void;
}
//# sourceMappingURL=connection.d.ts.map