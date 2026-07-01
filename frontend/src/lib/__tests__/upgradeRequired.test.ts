import { describe, expect, it } from 'vitest';

import { getUpgradeRequiredReason, isUpgradeRequiredError } from '../upgradeRequired';

describe('upgradeRequired', () => {
  it('detects Growth-only custom agent errors', () => {
    const reason = getUpgradeRequiredReason('Custom AI agents requires the Growth plan');

    expect(reason?.kind).toBe('custom_agents');
    expect(reason?.title).toBe('Upgrade to Growth');
    expect(isUpgradeRequiredError('Custom AI agents requires the Growth plan')).toBe(true);
  });

  it('detects Growth-only automation flow errors', () => {
    const reason = getUpgradeRequiredReason('Automation flows requires the Growth plan');

    expect(reason?.kind).toBe('automation_flows');
    expect(reason?.primaryBenefit).toBe('Automation flows');
  });

  it('detects AI usage exhaustion', () => {
    const reason = getUpgradeRequiredReason('AI usage exhausted');

    expect(reason?.kind).toBe('ai_usage');
    expect(reason?.message).toContain('AI usage');
  });

  it('detects Starter document limits as unlimited documents on Growth', () => {
    const reason = getUpgradeRequiredReason('Starter includes up to 500 documents');

    expect(reason?.kind).toBe('documents_limit');
    expect(reason?.primaryBenefit).toBe('Unlimited documents');
  });

  it('ignores ordinary validation errors', () => {
    expect(getUpgradeRequiredReason('Name is required')).toBeNull();
    expect(isUpgradeRequiredError(undefined)).toBe(false);
  });
});
