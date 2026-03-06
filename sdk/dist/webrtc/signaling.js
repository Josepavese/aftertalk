import { EventEmitter } from 'events';
export class SignalingClient extends EventEmitter {
    wsUrl;
    ws = null;
    constructor(wsUrl) {
        super();
        this.wsUrl = wsUrl;
    }
    async connect(sessionId, participantId, token) {
        const url = `${this.wsUrl}?sessionId=${sessionId}&participantId=${participantId}&token=${token}`;
        this.ws = new WebSocket(url);
        return new Promise((resolve, reject) => {
            this.ws.onopen = () => resolve();
            this.ws.onerror = (e) => reject(e);
            this.ws.onmessage = (e) => {
                const msg = JSON.parse(e.data);
                this.emit('message', msg);
            };
        });
    }
    send(message) {
        this.ws?.send(JSON.stringify(message));
    }
    disconnect() {
        this.ws?.close();
        this.ws = null;
    }
}
//# sourceMappingURL=signaling.js.map