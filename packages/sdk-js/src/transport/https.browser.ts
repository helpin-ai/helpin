import type { Transport } from '../core/types';
import type { Config } from '@/core/types';

export class HttpsTransport implements Transport {
  constructor(
    private trackingHost: string,
    private config: Config,
  ) {
    void this.trackingHost;
    void this.config;
  }

  async send(): Promise<void> {
    throw new Error(
      'HttpsTransport is not available in the browser SDK build.',
    );
  }
}
