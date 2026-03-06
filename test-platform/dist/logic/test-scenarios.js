export class TestScenarioExecutor {
    webrtc;
    _http;
    _reporter;
    _config;
    constructor(webrtc, _http, _reporter, _config) {
        this.webrtc = webrtc;
        this._http = _http;
        this._reporter = _reporter;
        this._config = _config;
    }
    async runConnectivityTests() {
        const results = [];
        results.push(await this.testServerHealth());
        results.push(await this.testWebRTCConnection());
        return results;
    }
    async testServerHealth() {
        const start = Date.now();
        try {
            return { name: 'server_health', status: 'passed', durationMs: Date.now() - start };
        }
        catch (error) {
            return { name: 'server_health', status: 'failed', durationMs: Date.now() - start, error: String(error) };
        }
    }
    async testWebRTCConnection() {
        const start = Date.now();
        try {
            await this.webrtc.connect('ws://localhost:8080/ws', 'token');
            await this.webrtc.disconnect();
            return { name: 'webrtc_connection', status: 'passed', durationMs: Date.now() - start };
        }
        catch (error) {
            return { name: 'webrtc_connection', status: 'failed', durationMs: Date.now() - start, error: String(error) };
        }
    }
}
//# sourceMappingURL=test-scenarios.js.map