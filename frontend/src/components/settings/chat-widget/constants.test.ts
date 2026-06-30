import { describe, expect, it } from 'vitest';
import { canRemoveHelpinBranding, ICON_OPTIONS } from './constants';

describe('chat widget settings constants', () => {
  it('uses distinct icons for launcher icon choices', () => {
    const iconComponents = ICON_OPTIONS.map((option) => option.icon);

    expect(new Set(iconComponents).size).toBe(iconComponents.length);
  });

  it('allows only Growth workspaces to remove Helpin branding', () => {
    expect(canRemoveHelpinBranding({ plan: 'growth', locked: false })).toBe(true);
    expect(canRemoveHelpinBranding({ plan: 'starter', locked: false })).toBe(false);
    expect(canRemoveHelpinBranding({ plan: 'founder', locked: false })).toBe(false);
    expect(canRemoveHelpinBranding({ plan: 'growth', locked: true })).toBe(false);
    expect(canRemoveHelpinBranding(null)).toBe(false);
  });
});
