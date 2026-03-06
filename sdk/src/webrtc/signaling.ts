import { EventEmitter } from 'events';

export class SignalingClient extends EventEmitter {
  private ws: WebSocket | null = null;

  constructor(private wsUrl: string) {
    super();
  }

  async connect(sessionId: string, participantId: string, token: string): Promise<void> {
    const url = `${this.wsUrl}?sessionId=${sessionId}&participantId=${participantId}&token=${token}`;
    this.ws = new WebSocket(url);
    
    return new Promise((resolve, reject) => {
      this.ws!.onopen = () => resolve();
      this.ws!.onerror = (e) => reject(e);
      this.ws!.onmessage = (e) => {
        const msg = JSON.parse(e.data);
        this.emit('message', msg);
      };
    });
  }

  send(message: object): void {
    this.ws?.send(JSON.stringify(message));
  }

  disconnect(): void {
    this.ws?.close();
    this.ws = null;
  }
}
