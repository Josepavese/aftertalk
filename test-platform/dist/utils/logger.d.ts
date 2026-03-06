export declare enum LogLevel {
    DEBUG = 0,
    INFO = 1,
    WARN = 2,
    ERROR = 3
}
export declare class NullLogger {
    debug(): void;
    info(): void;
    warn(): void;
    error(): void;
}
export declare class ConsoleLogger {
    private level;
    constructor(level?: LogLevel);
    debug(m: string): void;
    info(m: string): void;
    error(m: string): void;
    warn(m: string): void;
}
//# sourceMappingURL=logger.d.ts.map