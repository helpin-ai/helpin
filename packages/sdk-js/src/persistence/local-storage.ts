import { getLogger, Logger } from '../utils/logger';
import { isWindowAvailable } from '../utils/common';

export class LocalStoragePersistence {
  private storage: Record<string, any> = {};
  private prefix: string;
  private logger: Logger;
  private storageDisabled: boolean = false;
  private quotaErrorLogged: boolean = false;

  constructor(apiKey: string, logger?: Logger) {
    this.prefix = `helpin_${apiKey}_`;
    this.logger = logger || getLogger();
    this.load();
  }

  set(key: string, value: any): void {
    this.storage[key] = value;
    this.save();
  }

  get(key: string): any {
    return this.storage[key];
  }

  remove(key: string): void {
    delete this.storage[key];
    this.save();
  }

  clear(): void {
    this.storage = {};
    this.save();
  }

  isStorageEnabled(): boolean {
    return !this.storageDisabled;
  }

  private load(): void {
    if (!isWindowAvailable()) {
      this.logger.warn('localStorage is not available in this environment');
      return;
    }
    try {
      const data = localStorage.getItem(this.prefix + 'data');
      if (data) {
        this.storage = JSON.parse(data);
      }
    } catch (error: any) {
      if (this.isQuotaError(error)) {
        this.storageDisabled = true;
        this.logQuotaOnce(error);
      } else {
        this.logger.error('Error loading from localStorage:', error);
      }
    }
  }

  save(): boolean {
    if (!isWindowAvailable()) {
      this.logger.warn('localStorage is not available in this environment');
      return false;
    }
    if (this.storageDisabled) {
      return false;
    }

    try {
      localStorage.setItem(this.prefix + 'data', JSON.stringify(this.storage));
      return true;
    } catch (error: any) {
      if (this.isQuotaError(error)) {
        this.storageDisabled = true;
        this.logQuotaOnce(error);
      } else {
        this.logger.error('Error saving to localStorage:', error);
      }
      return false;
    }
  }

  private isQuotaError(error: any): boolean {
    const name = error?.name;
    return (
      name === 'QuotaExceededError' ||
      name === 'NS_ERROR_DOM_QUOTA_REACHED' ||
      error?.code === 22 ||
      error?.code === 1014
    );
  }

  private logQuotaOnce(error: any): void {
    if (this.quotaErrorLogged) return;
    this.quotaErrorLogged = true;
    this.logger.error(
      'localStorage quota exceeded; persisting disabled. Continuing with in-memory storage only.',
      error,
    );
  }
}
