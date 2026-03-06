import { EventEmitter } from 'events';
import { SDKConfig, Session, Participant, ILogger, LogLevel } from './types.js';
import { ConfigLoader } from './config.js';
import { ConsoleLogger } from './utils/logger.js';

export interface AftertalkClientOptions {
  config?: SDKConfig;
  configPath?: string;
}

export class AftertalkClient extends EventEmitter {
  private config: SDKConfig | null = null;
  private logger: ILogger;
  private currentSession: Session | null = null;

  constructor(options: AftertalkClientOptions = {}) {
    super();
    this.logger = new ConsoleLogger(LogLevel.INFO);
    
    if (options.config) {
      this.config = options.config;
    } else if (options.configPath) {
      const loader = new ConfigLoader(options.configPath);
      this.config = loader.load();
    } else {
      try {
        const loader = new ConfigLoader('./config.yaml');
        this.config = loader.load();
      } catch {
        this.config = null;
      }
    }

    if (this.config && this.config.sdk?.logging?.level === 'debug') {
      this.logger = new ConsoleLogger(LogLevel.DEBUG);
    }
  }

  async connect(): Promise<void> {
    this.logger.info('Connecting to Aftertalk');
    this.emit('connected');
  }

  disconnect(): void {
    this.logger.info('Disconnecting');
    this.emit('disconnected');
  }

  async createSession(title: string): Promise<Session> {
    this.logger.info('Creating session', { title });
    const session: Session = {
      id: 'session-' + Date.now(),
      title,
      status: 'pending',
      participants: []
    };
    this.currentSession = session;
    return session;
  }

  async joinSession(sessionId: string, name: string, role: 'host' | 'speaker' | 'listener' = 'speaker'): Promise<Participant> {
    this.logger.info('Joining session', { sessionId, name });
    return { id: 'p-' + Date.now(), name, role, sessionId };
  }

  async leaveSession(sessionId: string): Promise<void> {
    this.logger.info('Leaving session', { sessionId });
    this.currentSession = null;
  }

  getSession(): Session | null {
    return this.currentSession;
  }
}
