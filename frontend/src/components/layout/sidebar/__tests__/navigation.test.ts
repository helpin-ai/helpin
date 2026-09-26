import { describe, expect, it } from 'vitest';
import { isSidebarLinkActive } from '../navigation';

describe('automation tools sidebar navigation', () => {
  const toolsLink = '/w/acme/automation/tools';

  it('marks Tools active on the catalog route', () => {
    expect(isSidebarLinkActive(toolsLink, {}, toolsLink)).toBe(true);
  });

  it('keeps Tools active while the legacy Connections route redirects', () => {
    expect(isSidebarLinkActive(`${toolsLink}/connections`, {}, toolsLink)).toBe(true);
  });
});

it('highlights settings home only on the homepage', () => {
  expect(isSidebarLinkActive('/w/acme/settings', {}, '/w/acme/settings')).toBe(true);
  expect(isSidebarLinkActive('/w/acme/settings/crm-email', {}, '/w/acme/settings')).toBe(false);
});
