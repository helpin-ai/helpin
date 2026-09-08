import { describe, expect, it } from 'vitest';
import { CheckListIcon } from '@/lib/icons';
import { buildPanelNavGroups, buildRailItems, deriveActiveRail, projectCreateOptions, supportModuleUnreadCount, teamSubItems } from '../config';

describe('workspace rail navigation', () => {
  it('drives the Support indicator from the global personal unread total', () => {
    expect(supportModuleUnreadCount({ total: 4 })).toBe(4);
    expect(supportModuleUnreadCount({ total: 0 })).toBe(0);
    expect(supportModuleUnreadCount(undefined)).toBe(0);
  });

  it('uses the shared task icon for team task navigation', () => {
    expect(teamSubItems.find((item) => item.key === 'tasks')?.icon).toBe(CheckListIcon);
    expect(projectCreateOptions.find((item) => item.key === 'task')?.icon).toBe(CheckListIcon);
  });

  it('orders modules from projects through settings', () => {
    const items = buildRailItems('acme', 0);

    expect(items.map((item) => item.id)).toEqual([
      'projects',
      'support',
      'docs',
      'crm',
      'automation',
      'settings',
    ]);
  });

  it('opens CRM on Overview by default and accepts a remembered CRM section', () => {
    const defaults = buildRailItems('acme', 0);
    const remembered = buildRailItems('acme', 0, undefined, '/w/acme/crm/contacts');

    expect(defaults.find((item) => item.id === 'crm')?.defaultLink).toBe('/w/acme/crm/overview');
    expect(remembered.find((item) => item.id === 'crm')?.defaultLink).toBe('/w/acme/crm/contacts');
  });

  it('lists Overview before CRM records and intelligence pages', () => {
    const groups = buildPanelNavGroups('acme', false);
    expect(groups.crm[0]?.items.map((item) => item.label)).toEqual([
      'Overview',
      'Contacts',
      'Companies',
      'Deals',
      'Meetings',
      'Signals',
      'Playbooks',
    ]);
  });
});

describe('setup success rail navigation', () => {
  it('places the persistent setup guide after settings with progress and a separator', () => {
    const items = buildRailItems('acme', 0, 42);
    const settingsIndex = items.findIndex((item) => item.id === 'settings');
    const setupIndex = items.findIndex((item) => item.id === 'setup');

    expect(setupIndex).toBe(settingsIndex + 1);
    expect(items[setupIndex]).toMatchObject({
      label: 'Setup',
      defaultLink: '/w/acme/setup',
      progressPercent: 42,
      separatorBefore: true,
    });
  });

  it('treats the setup route as its own active rail', () => {
    expect(deriveActiveRail('/w/acme/setup')).toBe('setup');
  });
});

describe('settings team navigation', () => {
  it('nests available teams beneath the Teams settings item', () => {
    const groups = buildPanelNavGroups('acme', true, undefined, 0, [
      { id: 'team-platform', name: 'Platform' },
      { id: 'team-success', name: 'Customer Success' },
    ]);
    const workspace = groups.settings.find((group) => group.label === 'Workspace');
    const teams = workspace?.items.find((item) => item.label === 'Teams');

    expect(teams?.children).toEqual([
      { label: 'Platform', link: '/w/acme/settings/teams?team=team-platform' },
      { label: 'Customer Success', link: '/w/acme/settings/teams?team=team-success' },
    ]);
  });
});
