export class MockHTTPProvider {
    logger;
    constructor(logger) {
        this.logger = logger;
    }
    async get(path) {
        this.logger.debug(`GET ${path}`);
        return {};
    }
    async post(path, data) {
        this.logger.debug(`POST ${path}`, data);
        return {};
    }
}
//# sourceMappingURL=http-provider.js.map