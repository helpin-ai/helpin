export enum LogLevel {
  DEBUG,
  INFO,
  WARN,
  ERROR,
}

export class Logger {
  constructor(private level: LogLevel) {}

  debug(message: string, ...args: any[]): void {
    if (this.level <= LogLevel.DEBUG) {
      console.debug(`[Helpin Debug]:`, message, ...args);
    }
  }

  info(message: string, ...args: any[]): void {
    if (this.level <= LogLevel.INFO) {
      console.info(`[Helpin Info]:`, message, ...args);
    }
  }

  warn(message: string, ...args: any[]): void {
    if (this.level <= LogLevel.WARN) {
      console.warn(`[Helpin Warning]:`, message, ...args);
    }
  }

  error(message: string, ...args: any[]): void {
    if (this.level <= LogLevel.ERROR) {
      console.error(`[Helpin Error]:`, message, ...args);
    }
  }
}

export function getLogger(level: LogLevel = LogLevel.DEBUG): Logger {
  return new Logger(level);
}
