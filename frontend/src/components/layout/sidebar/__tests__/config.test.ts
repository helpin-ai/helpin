import { describe, expect, it } from 'vitest';
import { buildRailItems, deriveActiveRail } from '../config';

describe('workspace rail navigation', () => {
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
