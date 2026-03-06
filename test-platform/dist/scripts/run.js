import { TestScenarioExecutor } from '../logic/test-scenarios.js';
import { MockWebRTCProvider } from '../providers/webrtc-provider.js';
import { MockHTTPProvider } from '../providers/http-provider.js';
import { NullLogger } from '../utils/logger.js';
const logger = new NullLogger();
const webrtc = new MockWebRTCProvider(logger);
const http = new MockHTTPProvider(logger);
const config = {
    server: { host: 'localhost', port: 8080, healthPath: '/health', signalingPath: '/ws', timeout: 5000 },
    webrtc: { iceServers: [], audioSettings: { codec: 'opus', sampleRate: 48000, channels: 2, bitrate: 128000 } },
    audio: { samplePath: '', samples: [] },
    scenarios: []
};
const executor = new TestScenarioExecutor(webrtc, http, {
    report: async () => { },
    summary: async () => ({ total: 0, passed: 0, failed: 0, skipped: 0, durationMs: 0 })
}, config);
console.log('Running test platform...');
executor.runConnectivityTests().then((results) => {
    console.log('Results:', results);
});
//# sourceMappingURL=run.js.map