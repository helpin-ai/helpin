import { describe, expect, it } from 'vitest';
import { getSettingsSidebarGroups, SETTINGS_ROUTE_SECTIONS } from '../settingsSections';

function visibleSectionIDs(canManageSettings: boolean) {
  return getSettingsSidebarGroups(canManageSettings).flatMap((group) =>
    group.sections.map((section) => section.id),
  );
}

describe('getSettingsSidebarGroups', () => {
  it('shows billing even when workspace settings management is unavailable', () => {
    expect(visibleSectionIDs(false)).toContain('billing');
  });

  it('keeps billing in the workspace settings group', () => {
    const workspaceGroup = getSettingsSidebarGroups(false).find((group) => group.label === 'Workspace');

    expect(workspaceGroup?.sections.map((section) => section.id)).toContain('billing');
  });

  it('still hides settings-admin-only sections without workspace settings management', () => {
    expect(visibleSectionIDs(false)).not.toContain('command-intents');
  });

  it('places inbound and external MCP together after Access', () => {
    const workspaceGroup = getSettingsSidebarGroups(true, new Set(['workspace.read', 'settings.read', 'module_access.manage']))
      .find((group) => group.label === 'Workspace');

    const sections = workspaceGroup?.sections.map((section) => section.id) ?? [];
    expect(sections.indexOf('mcp')).toBe(sections.indexOf('access') + 1);
    expect(sections.indexOf('external-mcp')).toBe(sections.indexOf('mcp') + 1);
    expect(sections.indexOf('repositories')).toBe(sections.indexOf('external-mcp') + 1);
  });

  it('labels the inbound workspace surface MCP access', () => {
    expect(SETTINGS_ROUTE_SECTIONS.find((section) => section.id === 'mcp')).toMatchObject({
      label: 'MCP access',
      description: 'Connect outside MCP clients to Helpin and control their workspace access.',
    });
  });

  it('hides MCP without workspace read permission', () => {
    const sections = getSettingsSidebarGroups(true, new Set(['module_access.manage']))
      .flatMap((group) => group.sections.map((section) => section.id));

    expect(sections).not.toContain('mcp');
  });

  it('exposes outbound MCP only with settings read permission', () => {
    const withoutSettingsRead = getSettingsSidebarGroups(true, new Set(['workspace.read']))
      .flatMap((group) => group.sections.map((section) => section.id));
    const withSettingsRead = getSettingsSidebarGroups(true, new Set(['workspace.read', 'settings.read']))
      .flatMap((group) => group.sections.map((section) => section.id));

    expect(withoutSettingsRead).not.toContain('external-mcp');
    expect(withSettingsRead).toContain('external-mcp');
  });

  it('puts AI Assistant first in support settings', () => {
    const supportGroup = getSettingsSidebarGroups(true).find((group) => group.label === 'Support');

    expect(supportGroup?.sections.map((section) => section.id).slice(0, 3)).toEqual([
      'support-ai-assistant',
      'inboxes-routing',
      'chat-general',
    ]);
  });
});
