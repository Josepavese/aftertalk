import { EventEmitter } from 'events';
export class WebRTCConnection extends EventEmitter {
    iceServers;
    peerConnection = null;
    constructor(iceServers = []) {
        super();
        this.iceServers = iceServers;
    }
    async connect() {
        const config = {
            iceServers: this.iceServers.map(s => ({ urls: s.urls, username: s.username, credential: s.credential }))
        };
        this.peerConnection = new RTCPeerConnection(config);
        this.peerConnection.onicecandidate = (event) => {
            if (event.candidate) {
                this.emit('icecandidate', event.candidate);
            }
        };
        this.peerConnection.ontrack = (event) => {
            this.emit('track', event);
        };
        this.emit('connected');
    }
    async createOffer() {
        if (!this.peerConnection)
            throw new Error('Not connected');
        const offer = await this.peerConnection.createOffer();
        await this.peerConnection.setLocalDescription(offer);
        return offer;
    }
    async handleOffer(offer) {
        if (!this.peerConnection)
            throw new Error('Not connected');
        await this.peerConnection.setRemoteDescription(new RTCSessionDescription(offer));
        const answer = await this.peerConnection.createAnswer();
        await this.peerConnection.setLocalDescription(answer);
        return answer;
    }
    close() {
        this.peerConnection?.close();
        this.peerConnection = null;
    }
}
//# sourceMappingURL=connection.js.map