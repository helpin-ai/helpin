import { describe, expect, it } from 'vitest';

import { getWorkspaceBillingBadgeClasses } from '../WorkspaceSelector';

describe('WorkspaceSelector billing badge presentation', () => {
  it('keeps the billing badge text away from the badge border', () => {
    const className = getWorkspaceBillingBadgeClasses('bg-amber-500/10 text-amber-700');

    expect(className).toContain('px-2.5');
    expect(className).toContain('py-1');
    expect(className).toContain('leading-none');
  });

  it('gives trial labels enough room before truncating', () => {
    const className = getWorkspaceBillingBadgeClasses('bg-amber-500/10 text-amber-700');

    expect(className).toContain('max-w-[12.5rem]');
  });
});
