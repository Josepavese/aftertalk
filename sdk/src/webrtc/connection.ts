import { EventEmitter } from 'events';
import { ICEServer } from '../types.js';

export class WebRTCConnection extends EventEmitter {
  private peerConnection: RTCPeerConnection | null = null;

  constructor(private iceServers: ICEServer[] = []) {
    super();
  }

  async connect(): Promise<void> {
    const config: RTCConfiguration = {
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

  async createOffer(): Promise<RTCSessionDescriptionInit> {
    if (!this.peerConnection) throw new Error('Not connected');
    const offer = await this.peerConnection.createOffer();
    await this.peerConnection.setLocalDescription(offer);
    return offer;
  }

  async handleOffer(offer: RTCSessionDescriptionInit): Promise<RTCSessionDescriptionInit> {
    if (!this.peerConnection) throw new Error('Not connected');
    await this.peerConnection.setRemoteDescription(new RTCSessionDescription(offer));
    const answer = await this.peerConnection.createAnswer();
    await this.peerConnection.setLocalDescription(answer);
    return answer;
  }

  close(): void {
    this.peerConnection?.close();
    this.peerConnection = null;
  }
}
