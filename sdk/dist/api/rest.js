import axios from 'axios';
export class RESTClient {
    client;
    constructor(config) {
        this.client = axios.create({
            baseURL: config.sdk.api.baseUrl + config.sdk.api.apiPrefix,
            timeout: config.sdk.api.timeout
        });
    }
    setAuthToken(token) {
        this.client.defaults.headers.common['Authorization'] = `Bearer ${token}`;
    }
    async createSession(data) {
        const res = await this.client.post('/sessions', data);
        return res.data;
    }
    async getSession(sessionId) {
        const res = await this.client.get(`/sessions/${sessionId}`);
        return res.data;
    }
    async joinSession(sessionId, data) {
        const res = await this.client.post(`/sessions/${sessionId}/participants`, data);
        return res.data;
    }
    async getTranscriptions(sessionId) {
        const res = await this.client.get(`/sessions/${sessionId}/transcriptions`);
        return res.data;
    }
    async requestMinutes(sessionId) {
        const res = await this.client.post(`/sessions/${sessionId}/minutes/generate`);
        return res.data;
    }
}
//# sourceMappingURL=rest.js.map