import { describe, expect, it } from 'vitest';
import { isSidebarLinkActive } from '../navigation';

describe('automation tools sidebar navigation', () => {
  const toolsLink = '/w/acme/automation/tools';

  it('marks Tools active on the catalog route', () => {
    expect(isSidebarLinkActive(toolsLink, {}, toolsLink)).toBe(true);
  });

  it('keeps Tools active on the nested Connections route', () => {
    expect(isSidebarLinkActive(`${toolsLink}/connections`, {}, toolsLink)).toBe(true);
  });
});
