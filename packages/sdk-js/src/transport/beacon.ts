import { Transport } from '../core/types';
import { getLogger } from '../utils/logger';
import { isWindowAvailable, generateRandom } from '../utils/common';
import { Config } from '@/core/types';

export class BeaconTransport implements Transport {
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
    const url = this.constructUrl(apiKey);
    const blob = new Blob([JSON.stringify(payloads)], {
      type: 'application/json',
    });

    if (navigator.sendBeacon(url, blob)) {
      this.logger.debug(
        `Successfully queued ${payloads.length} event(s) via Beacon API`,
      );
    } else {
      throw new Error('Failed to queue events via Beacon API');
    }

    // Note: Beacon API doesn't provide a way to handle the response,
    // so we can't implement post-handling here.
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

  // Note: Beacon API doesn't support custom headers, so we can't use them here.
  // If custom headers are crucial, you might want to fall back to XHR or Fetch in those cases.
}
