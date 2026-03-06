export enum LogLevel {
  DEBUG = 0,
  INFO = 1,
  WARN = 2,
  ERROR = 3
}

export class NullLogger {
  debug() {}
  info() {}
  warn() {}
  error() {}
}

export class ConsoleLogger {
  constructor(private level: LogLevel = LogLevel.INFO) {}
  debug(m: string) { if (this.level <= LogLevel.DEBUG) console.log('DEBUG:', m); }
  info(m: string) { if (this.level <= LogLevel.INFO) console.log('INFO:', m); }
  error(m: string) { console.log('ERROR:', m); }
  warn(m: string) { console.log('WARN:', m); }
}
