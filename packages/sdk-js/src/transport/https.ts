import { Transport } from '../core/types';
import { getLogger } from '../utils/logger';
import { Config } from '@/core/types';
import * as https from 'https';
import * as url from 'url';

export class HttpsTransport implements Transport {
  private config: Config;

  constructor(
    private trackingHost: string,
    config: Config,
    private logger = getLogger(),
  ) {
    this.config = config;
  }

  async send(payloads: any[]): Promise<void> {
    const apiKey = this.config.key;
    const urlObject = new url.URL(this.constructUrl(apiKey));

    const options = {
      hostname: urlObject.hostname,
      port: 443,
      path: `${urlObject.pathname}${urlObject.search}`,
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...this.getCustomHeaders(),
      },
    };

    return new Promise<void>((resolve, reject) => {
      const req = https.request(options, (res) => {
        let data = '';

        res.on('data', (chunk) => {
          data += chunk;
        });

        res.on('end', () => {
          const statusCode = res.statusCode || 0; // Default to 0 if undefined
          if (statusCode >= 200 && statusCode < 300) {
            this.logger.debug(`Successfully sent ${payloads.length} event(s)`);
            resolve();
          } else {
            reject(new Error(`HTTP error! status: ${statusCode}`));
          }
        });
      });

      req.on('error', (error) => {
        reject(error);
      });

      req.write(JSON.stringify(payloads));
      req.end();
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
    return `${this.trackingHost}/api/v1/s2s/event?token=${apiKey}${cookiePolicy}${ipPolicy}`;
  }

  private getCustomHeaders(): Record<string, string> {
    if (typeof this.config.customHeaders === 'function') {
      return this.config.customHeaders();
    } else if (this.config.customHeaders) {
      return this.config.customHeaders;
    }
    return {};
  }
}
