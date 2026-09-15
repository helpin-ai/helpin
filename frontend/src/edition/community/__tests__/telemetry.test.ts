import { describe, expect, it } from 'vitest';
import * as config from '../config';
import { initializeAppAnalytics } from '@/lib/analytics';

describe('Community telemetry defaults', () => {
 it('has no analytics or hosted widget defaults, even on the SaaS hostname', () => {
  expect(config.APP_ANALYTICS_HOST).toBe('');
  expect(config.USERMAVEN_KEY).toBe('');
  expect(config.CUSTOMER_IO_WRITE_KEY).toBe('');
  expect(config.defaultSupportWidgetKey).toBe('');
  expect(config.defaultSupportWidgetHost).toBe('');
  expect(initializeAppAnalytics({ hostname:'app.helpin.ai' })).toBe(false);
 });
});
