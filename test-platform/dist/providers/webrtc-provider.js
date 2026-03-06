export class MockWebRTCProvider {
    logger;
    connected = false;
    constructor(logger) {
        this.logger = logger;
    }
    async connect(url, _token) {
        this.logger.info(`Connecting to ${url}`);
        this.connected = true;
    }
    async disconnect() {
        this.connected = false;
    }
    isConnected() {
        return this.connected;
    }
}
//# sourceMappingURL=webrtc-provider.js.map