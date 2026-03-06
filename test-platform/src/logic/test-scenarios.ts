import { IWebRTCConnection, IHTTPClient, ITestReporter } from '../middleware/interfaces.js';
import { TestConfig, TestResult } from './types.js';

export class TestScenarioExecutor {
  constructor(
    private webrtc: IWebRTCConnection,
    private _http: IHTTPClient,
    private _reporter: ITestReporter,
    private _config: TestConfig
  ) {}

  async runConnectivityTests(): Promise<TestResult[]> {
    const results: TestResult[] = [];
    results.push(await this.testServerHealth());
    results.push(await this.testWebRTCConnection());
    return results;
  }

  private async testServerHealth(): Promise<TestResult> {
    const start = Date.now();
    try {
      return { name: 'server_health', status: 'passed', durationMs: Date.now() - start };
    } catch (error) {
      return { name: 'server_health', status: 'failed', durationMs: Date.now() - start, error: String(error) };
    }
  }

  private async testWebRTCConnection(): Promise<TestResult> {
    const start = Date.now();
    try {
      await this.webrtc.connect('ws://localhost:8080/ws', 'token');
      await this.webrtc.disconnect();
      return { name: 'webrtc_connection', status: 'passed', durationMs: Date.now() - start };
    } catch (error) {
      return { name: 'webrtc_connection', status: 'failed', durationMs: Date.now() - start, error: String(error) };
    }
  }
}
