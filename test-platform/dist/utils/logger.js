export var LogLevel;
(function (LogLevel) {
    LogLevel[LogLevel["DEBUG"] = 0] = "DEBUG";
    LogLevel[LogLevel["INFO"] = 1] = "INFO";
    LogLevel[LogLevel["WARN"] = 2] = "WARN";
    LogLevel[LogLevel["ERROR"] = 3] = "ERROR";
})(LogLevel || (LogLevel = {}));
export class NullLogger {
    debug() { }
    info() { }
    warn() { }
    error() { }
}
export class ConsoleLogger {
    level;
    constructor(level = LogLevel.INFO) {
        this.level = level;
    }
    debug(m) { if (this.level <= LogLevel.DEBUG)
        console.log('DEBUG:', m); }
    info(m) { if (this.level <= LogLevel.INFO)
        console.log('INFO:', m); }
    error(m) { console.log('ERROR:', m); }
    warn(m) { console.log('WARN:', m); }
}
//# sourceMappingURL=logger.js.map