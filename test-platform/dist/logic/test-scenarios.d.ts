import { IWebRTCConnection, IHTTPClient, ITestReporter } from '../middleware/interfaces.js';
import { TestConfig, TestResult } from './types.js';
export declare class TestScenarioExecutor {
    private webrtc;
    private _http;
    private _reporter;
    private _config;
    constructor(webrtc: IWebRTCConnection, _http: IHTTPClient, _reporter: ITestReporter, _config: TestConfig);
    runConnectivityTests(): Promise<TestResult[]>;
    private testServerHealth;
    private testWebRTCConnection;
}
//# sourceMappingURL=test-scenarios.d.ts.map