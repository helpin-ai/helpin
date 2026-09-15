// @vitest-environment jsdom
import { describe, expect, it, beforeEach } from 'vitest';
import { getSettingsSidebarGroups } from '../settingsSections';
import { searchSettings, readRecentSettings, rememberSetting, revealSettingOption } from '../settingsDiscovery';

const groups = getSettingsSidebarGroups(true, new Set(['workspace.read', 'settings.read', 'module_access.manage']));
beforeEach(() => { localStorage.clear(); document.body.innerHTML = ''; });
describe('settings discovery', () => {
  it('finds options using everyday terms and ranks the option first', () => {
    expect(searchSettings(groups, 'sending limit')[0]).toMatchObject({ sectionId: 'crm-email', optionId: 'email-sending-limits' });
    expect(searchSettings(groups, 'gmail signature')[0]).toMatchObject({ optionId: 'email-signature' });
    expect(searchSettings(groups, 'SIGNATURE')[0]).toMatchObject({ optionId: 'email-signature' });
    expect(searchSettings(groups, 'invite teammate')[0]).toMatchObject({ sectionId: 'members' });
    expect(searchSettings(groups, 'custom domain')[0]).toMatchObject({ sectionId: 'helpcenter', optionId: 'helpcenter-domain' });
  });
  it('never returns sections excluded from the visible directory', () => {
    const limited = getSettingsSidebarGroups(false, new Set());
    expect(searchSettings(limited, 'external mcp')).toEqual([]);
  });
  it('returns no false matches for an unknown query', () => {
    expect(searchSettings(groups, 'xyznotasetting')).toEqual([]);
  });
  it('scopes recent settings per viewer and workspace, deduplicates and caps at four', () => {
    for (const section of ['profile', 'members', 'teams', 'general', 'crm-email', 'members'] as const) rememberSetting('viewer:ws', section);
    expect(readRecentSettings('viewer:ws')).toEqual(['members', 'crm-email', 'general', 'teams']);
    expect(readRecentSettings('someone-else:ws')).toEqual([]);
  });
  it('opens collapsed ancestors of a search target without replacing form state', () => {
    document.body.innerHTML = '<details><summary>Mailbox</summary><details data-settings-option="email-signature"><summary>Signature</summary><input value="Unsaved signature" /></details></details>';
    expect(revealSettingOption('email-signature')).toBe(true);
    expect([...document.querySelectorAll('details')].every(el => el.open)).toBe(true);
    expect(document.querySelector('input')?.value).toBe('Unsaved signature');
  });
});
