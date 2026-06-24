import { describe, expect, it } from 'vitest';
import { getSettingsSidebarGroups } from '../settingsSections';

function visibleSectionIDs(canManageSettings: boolean) {
  return getSettingsSidebarGroups(canManageSettings).flatMap((group) =>
    group.sections.map((section) => section.id),
  );
}

describe('getSettingsSidebarGroups', () => {
  it('shows billing even when workspace settings management is unavailable', () => {
    expect(visibleSectionIDs(false)).toContain('billing');
  });

  it('still hides settings-admin-only sections without workspace settings management', () => {
    expect(visibleSectionIDs(false)).not.toContain('command-intents');
  });
});
