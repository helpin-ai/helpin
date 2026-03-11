import { Transport } from '../core/types';
import { getLogger, Logger } from '../utils/logger';
import { isWindowAvailable, generateRandom } from '../utils/common';
import { Config } from '@/core/types';

export class FetchTransport implements Transport {
  private config: Config;

  constructor(
    private trackingHost: string,
    config: Config,
    private logger: Logger = getLogger(),
  ) {
    this.config = config;
  }

  async send(payloads: any[]): Promise<void> {
    const apiKey = this.config.key;
    const url = this.constructUrl(apiKey);
    const body = JSON.stringify(payloads);

    const headers = {
      'Content-Type': 'application/json',
      ...this.getCustomHeaders(),
    };

    const response = await fetch(url, {
      method: 'POST',
      headers: headers,
      body: body,
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    this.logger.debug(`Successfully sent ${payloads.length} event(s)`);

    // Post-handling
    this.postHandle(response.status, await response.text());
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
