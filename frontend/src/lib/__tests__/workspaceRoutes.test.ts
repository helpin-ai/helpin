import { describe, expect, it } from 'vitest';

import { isWorkspaceSupportRoute } from '../workspaceRoutes';

describe('workspace route helpers', () => {
  it('matches the real support module routes', () => {
    expect(isWorkspaceSupportRoute('/w/test-docs/support')).toBe(true);
    expect(isWorkspaceSupportRoute('/w/test-docs/support/conv-1')).toBe(true);
  });

  it('does not treat support-named settings pages as support module routes', () => {
    expect(isWorkspaceSupportRoute('/w/test-docs/settings/support-ai-assistant')).toBe(false);
    expect(isWorkspaceSupportRoute('/w/test-docs/settings/inboxes-routing')).toBe(false);
  });
});
