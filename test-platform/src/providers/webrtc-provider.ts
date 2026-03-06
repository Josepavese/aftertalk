import { IWebRTCConnection, ILogger } from '../middleware/interfaces.js';

export class MockWebRTCProvider implements IWebRTCConnection {
  private connected = false;
  constructor(private logger: ILogger) {}

  async connect(url: string, _token: string): Promise<void> {
    this.logger.info(`Connecting to ${url}`);
    this.connected = true;
  }

  async disconnect(): Promise<void> {
    this.connected = false;
  }

  isConnected(): boolean {
    return this.connected;
  }
}
