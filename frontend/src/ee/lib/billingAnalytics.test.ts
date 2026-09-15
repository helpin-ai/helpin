import { expect, it } from 'vitest';
import { buildAnalyticsBillingEventProperties } from './billingAnalytics';
import type { WorkspaceBillingSummary } from '@/lib/types';
it('keeps billing event identity and usage in the commercial integration', () => {
  const billing = { plan: 'growth', status: 'active', ai_usage_used_microusd: 1200, pricing_version: 'version-1' } as WorkspaceBillingSummary;
  expect(buildAnalyticsBillingEventProperties('checkout_started', 'ws', billing)).toMatchObject({
    event_name: 'checkout_started', workspace_id: 'ws', plan: 'growth', billing_status: 'active', ai_usage_used_microusd: 1200, pricing_version: 'version-1',
  });
});
