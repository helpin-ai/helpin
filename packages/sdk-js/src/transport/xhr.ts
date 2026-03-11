import { Transport } from '../core/types';
import { getLogger } from '../utils/logger';
import { generateRandom, isWindowAvailable } from '../utils/common';
import { Config } from '@/core/types';

export class XhrTransport implements Transport {
  private config: Config;

  constructor(
    private trackingHost: string,
    config: Config,
    private logger = getLogger(),
  ) {
    this.config = config;
  }

  send(payloads: any[]): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      const apiKey = this.config.key;

      const url = this.constructUrl(apiKey);

      xhr.open('POST', url, true);
      xhr.setRequestHeader('Content-Type', 'application/json');

      // Set custom headers
      const customHeaders = this.getCustomHeaders();
      Object.keys(customHeaders).forEach((key) => {
        xhr.setRequestHeader(key, customHeaders[key]);
      });

      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          this.logger.debug(`Successfully sent ${payloads.length} event(s)`);
          resolve();
        } else {
          reject(new Error(`HTTP error! status: ${xhr.status}`));
        }
      };

      xhr.onerror = () => {
        reject(new Error('Network error'));
      };

      xhr.send(JSON.stringify(payloads));
    });
  }

  private constructUrl(apiKey: string): string {
    const cookiePolicy =
      this.config.cookiePolicy !== 'keep'
        ? `&cookie_policy=${this.config.cookiePolicy}`
        : '';
    const ipPolicy =
      this.config.ipPolicy !== 'keep'
        ? `&ip_policy=${this.config.ipPolicy}`
        : '';
    const urlPrefix = isWindowAvailable()
      ? '/api/v1/event'
      : '/api/v1/s2s/event';

    if (this.config.randomizeUrl) {
      return `${this.trackingHost}/api.${generateRandom()}?p_${generateRandom()}=${apiKey}${cookiePolicy}${ipPolicy}`;
    } else {
      return `${this.trackingHost}${urlPrefix}?token=${apiKey}${cookiePolicy}${ipPolicy}`;
    }
  }

  private getCustomHeaders(): Record<string, string> {
    if (typeof this.config.customHeaders === 'function') {
      return this.config.customHeaders();
    } else if (this.config.customHeaders) {
      return this.config.customHeaders;
    }
    return {};
  }

  private postHandle(code: number, body: string): void {
    // Implement post-handling logic if needed
    this.logger.debug(`Response received. Status: ${code}, Body: ${body}`);
  }
}
