import { EventEmitter } from 'events';
import { LogLevel } from './types.js';
import { ConfigLoader } from './config.js';
import { ConsoleLogger } from './utils/logger.js';
export class AftertalkClient extends EventEmitter {
    config = null;
    logger;
    currentSession = null;
    constructor(options = {}) {
        super();
        this.logger = new ConsoleLogger(LogLevel.INFO);
        if (options.config) {
            this.config = options.config;
        }
        else if (options.configPath) {
            const loader = new ConfigLoader(options.configPath);
            this.config = loader.load();
        }
        else {
            try {
                const loader = new ConfigLoader('./config.yaml');
                this.config = loader.load();
            }
            catch {
                this.config = null;
            }
        }
        if (this.config && this.config.sdk?.logging?.level === 'debug') {
            this.logger = new ConsoleLogger(LogLevel.DEBUG);
        }
    }
    async connect() {
        this.logger.info('Connecting to Aftertalk');
        this.emit('connected');
    }
    disconnect() {
        this.logger.info('Disconnecting');
        this.emit('disconnected');
    }
    async createSession(title) {
        this.logger.info('Creating session', { title });
        const session = {
            id: 'session-' + Date.now(),
            title,
            status: 'pending',
            participants: []
        };
        this.currentSession = session;
        return session;
    }
    async joinSession(sessionId, name, role = 'speaker') {
        this.logger.info('Joining session', { sessionId, name });
        return { id: 'p-' + Date.now(), name, role, sessionId };
    }
    async leaveSession(sessionId) {
        this.logger.info('Leaving session', { sessionId });
        this.currentSession = null;
    }
    getSession() {
        return this.currentSession;
    }
}
//# sourceMappingURL=client.js.map