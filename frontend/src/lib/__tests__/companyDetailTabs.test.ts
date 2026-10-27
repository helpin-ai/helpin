import { describe, expect, it } from 'vitest';
import { normalizeCompanyDetailTab, visibleCompanyDetailTab } from '../companyDetailTabs';

describe('company detail tabs', () => {
  it('accepts the portal tab from the URL', () => {
    expect(normalizeCompanyDetailTab('portal')).toBe('portal');
    expect(normalizeCompanyDetailTab('unknown')).toBe('overview');
  });

  it('shows the portal tab only to support admins once access is known', () => {
    expect(visibleCompanyDetailTab('portal', true, true)).toBe('portal');
    expect(visibleCompanyDetailTab('portal', false, true)).toBe('overview');
    // While access loads, keep the requested tab rather than flashing the overview.
    expect(visibleCompanyDetailTab('portal', false, false)).toBe('portal');
    expect(visibleCompanyDetailTab('support', false, true)).toBe('support');
  });
});
