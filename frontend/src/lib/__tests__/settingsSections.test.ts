import { billingEnabled } from '@edition/config';
import { describe, expect, it } from 'vitest';
import { getSettingsSidebarGroups, SETTINGS_ROUTE_SECTIONS } from '../settingsSections';

function visibleSectionIDs(canManageSettings: boolean) {
  return getSettingsSidebarGroups(canManageSettings).flatMap((group) =>
    group.sections.map((section) => section.id),
  );
}

describe('getSettingsSidebarGroups', () => {
  it.skipIf(billingEnabled)('omits billing in community workspaces', () => {
    expect(SETTINGS_ROUTE_SECTIONS.map(section => section.id)).not.toContain('billing');
    expect(visibleSectionIDs(true)).not.toContain('billing');
    expect(visibleSectionIDs(false)).not.toContain('billing');
  });
  it.skipIf(!billingEnabled)('shows billing even when workspace settings management is unavailable', () => {
    expect(visibleSectionIDs(false)).toContain('billing');
  });

  it.skipIf(!billingEnabled)('keeps billing in the workspace settings group', () => {
    const workspaceGroup = getSettingsSidebarGroups(false).find((group) => group.label === 'Workspace');

    expect(workspaceGroup?.sections.map((section) => section.id)).toContain('billing');
  });

  it('still hides settings-admin-only sections without workspace settings management', () => {
    expect(visibleSectionIDs(false)).not.toContain('command-intents');
  });

  it('groups MCP and repositories under Integrations', () => {
    const workspaceGroup = getSettingsSidebarGroups(true, new Set(['workspace.read', 'settings.read', 'module_access.manage']))
      .find((group) => group.label === 'Integrations & data');

    const sections = workspaceGroup?.sections.map((section) => section.id) ?? [];
    expect(sections).toContain('mcp');
    expect(sections.indexOf('external-mcp')).toBe(sections.indexOf('repositories') + 1);
    expect(sections.indexOf('mcp')).toBe(sections.indexOf('external-mcp') + 1);
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

  it('puts inbox and widget setup before AI assistant', () => {
    const supportGroup = getSettingsSidebarGroups(true).find((group) => group.label === 'Support');

    expect(supportGroup?.sections.map((section) => section.id).slice(0, 3)).toEqual([
      'inboxes-routing',
      'chat-general',
      'support-ai-assistant',
    ]);
  });
  it('gates the AI settings sections on their read permissions', () => {
    const none = getSettingsSidebarGroups(true, new Set())
      .flatMap((group) => group.sections.map((section) => section.id));
    const both = getSettingsSidebarGroups(true, new Set(['workspace.read', 'settings.read']))
      .flatMap((group) => group.sections.map((section) => section.id));

    expect(none).not.toContain('ai-connections');
    expect(none).not.toContain('ai');
    expect(both).toContain('ai-connections');
    expect(both).toContain('ai');
  });

  it('gives each AI section its own icon', () => {
    const sections = getSettingsSidebarGroups(true, new Set(['workspace.read', 'settings.read']))
      .flatMap((group) => group.sections);
    const personal = sections.find((section) => section.id === 'ai-connections');
    const workspace = sections.find((section) => section.id === 'ai');
    const profile = sections.find((section) => section.id === 'profile');

    expect(personal?.icon).not.toBe(workspace?.icon);
    expect(personal?.icon).not.toBe(profile?.icon);
    expect(workspace?.icon).not.toBe(profile?.icon);
  });
});
