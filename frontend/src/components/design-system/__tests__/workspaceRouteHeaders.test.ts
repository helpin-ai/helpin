import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const source = (path: string) => readFileSync(resolve(__dirname, path), 'utf8');

describe('workspace route header contract', () => {
  it.each([
    ['Settings page frame', '../../../pages/settings/SettingsPageFrame.tsx'],
    ['Profile', '../../../pages/Profile.tsx'],
    ['Security', '../../../pages/SecuritySettings.tsx'],
    ['Organization', '../../../pages/AccountSettings.tsx'],
    ['Notifications', '../../../pages/NotificationSettings.tsx'],
    ['Git Connections', '../../settings/OrgGitConnectionsTab.tsx'],
    ['All Docs', '../../../pages/docs/DocsHome.tsx'],
    ['document lists', '../../../pages/docs/DocsDocumentList.tsx'],
  ])('uses QuietPageHeader for %s', (_label, path) => {
    expect(source(path)).toContain('<QuietPageHeader');
  });

  it.each([
    ['All Docs', '../../../pages/docs/DocsHome.tsx'],
    ['Docs document lists', '../../../pages/docs/DocsDocumentList.tsx'],
    ['CRM Overview', '../../../pages/crm/Overview.tsx'],
    ['CRM Signal inbox', '../../../pages/crm/Insights.tsx'],
    ['CRM Review', '../../../pages/crm/ReviewFeed.tsx'],
    ['CRM Contacts', '../../../pages/crm/Contacts.tsx'],
    ['CRM Companies', '../../../pages/crm/Companies.tsx'],
    ['CRM Deals', '../../../pages/crm/Deals.tsx'],
    ['CRM Meetings', '../../../pages/crm/Meetings.tsx'],
  ])('uses the compact shell header for %s', (_label, path) => {
    const pageSource = source(path);
    expect(pageSource).toContain('<QuietPageHeader');
    expect(pageSource).toContain('variant="shell"');
  });

  it.each([
    ['Automation Flows', '../../../pages/automation/AutomationFlows.tsx'],
    ['Automation Activity', '../../../pages/automation/AutomationActivity.tsx'],
    ['Automation Agents', '../../../pages/automation/Agents.tsx'],
    ['Automation Trigger Catalog', '../../../pages/automation/AutomationLibrary.tsx'],
    ['Automation Skill Catalog', '../../../pages/automation/SkillCatalog.tsx'],
    ['Automation Tools', '../../../pages/automation/AutomationToolsLayout.tsx'],
  ])('uses the shared compact Automation shell for %s', (_label, path) => {
    expect(source(path)).toContain('<AutomationShell');
  });

  it('keeps the Automation shell header above its scrolling page viewport', () => {
    const shellSource = source('../../automation/AutomationShell.tsx');
    expect(shellSource).toContain('<QuietPageHeader variant="shell"');
    expect(shellSource).toContain('<QuietPageViewport className="min-h-0 flex-1">');
    expect(shellSource.indexOf('<QuietPageHeader')).toBeLessThan(shellSource.indexOf('<QuietPageViewport'));
  });

  it.each([
    ['Teams detail', '../../settings/TeamsTab.tsx'],
  ])('keeps the specialized %s toolbar clear of the collapsed sidebar opener', (_label, path) => {
    expect(source(path)).toContain('workspaceSidebarSafeInsetClassName');
  });

  it.each([
    ['Docs editor', '../../../pages/docs/DocsDocumentDetail.tsx'],
    ['Docs space and collection pages', '../../../pages/docs/spaceDetail/SpaceNodeHeader.tsx'],
    ['Meeting detail', '../../../pages/crm/MeetingDetail.tsx'],
    ['Contact detail', '../../crm/contact-detail/ContactHeader.tsx'],
    ['Company detail', '../../../pages/crm/CompanyDetail.tsx'],
    ['Epic detail', '../../../pages/pm/EpicDetail.tsx'],
    ['Task detail sheet', '../../pm/TaskDetailPanel.tsx'],
  ])('uses the shared detail header contract for %s', (_label, path) => {
    expect(source(path)).toContain('<QuietDetailHeader');
    expect(source(path)).toContain('<QuietBreadcrumbs');
  });

  it('keeps the Epic identity and actions in the shared detail header', () => {
    const epicSource = source('../../../pages/pm/EpicDetail.tsx');
    const followButtonSource = source('../../notifications/FollowButton.tsx');

    expect(epicSource).toContain('presentation="header"');
    expect(epicSource).toContain('<QuietMetaLine');
    expect(epicSource).toContain('<QuietStatusBadge className="lg:hidden"');
    expect(epicSource).toContain('presentation="quiet"');
    expect(epicSource).toContain('presentation="detail-header"');
    expect(epicSource).toContain('<QuietDetailAction');
    expect(epicSource).not.toContain('ui-divider-bottom-fade');
    expect(epicSource.match(/aria-label="Epic title"/g)).toHaveLength(1);

    expect(followButtonSource).toContain("presentation?: 'default' | 'detail-header'");
    expect(followButtonSource).toContain("presentation === 'detail-header'");
    expect(followButtonSource).toContain('<QuietDetailAction');
  });

  it('places semantic status after metadata while keeping save state beneath actions', () => {
    const quietSource = source('../quiet.tsx');

    expect(quietSource).toContain("{meta || status ? (");
    expect(quietSource.indexOf('{meta ? <div')).toBeLessThan(quietSource.indexOf('{status ? <div'));
    expect(quietSource).toContain('flex shrink-0 flex-nowrap items-center gap-x-3');
    expect(quietSource).toContain('flex-col items-end gap-0.5');
    expect(quietSource).toContain('max-w-28 flex-nowrap');
  });

  it('renders the private Docs title in the header instead of repeating it in the editor', () => {
    const docsSource = source('../../../pages/docs/DocsDocumentDetail.tsx');
    expect(docsSource).toContain('<QuietTitleTextarea');
    expect(docsSource).toContain('presentation="header"');
    expect(docsSource).toContain('allowTitleWrap');
    expect(docsSource).toContain('pageScrollOnMobile');
    expect(docsSource).toContain('iconOnly');
    expect(docsSource).toContain('className="relative z-30"');
    expect(docsSource).toContain('docs-editor-standard-width relative isolate');
    expect(docsSource).toContain('lg:w-auto lg:flex-1 xl:overflow-clip');
    expect(docsSource).not.toContain('className="relative z-30 bg-background"');
    expect(docsSource.indexOf('headerPresencePeople.length > 0')).toBeGreaterThan(docsSource.indexOf('actions={('));
    expect(docsSource.indexOf('headerPresencePeople.length > 0')).toBeLessThan(docsSource.indexOf('label="Preview"'));
    expect(docsSource).toContain('flex flex-col items-end gap-0.5 sm:flex-row sm:items-center sm:gap-3');
    expect(docsSource).toContain('hidden text-[11.5px] font-medium sm:inline');
    expect(docsSource).toContain('order-first flex items-center gap-1.5 sm:order-none sm:gap-3');
    expect(docsSource).toContain('showTitle={false}');
  });

  it('preserves the established Docs publication-state typography and colors', () => {
    const docsSource = source('../../../pages/docs/DocsDocumentDetail.tsx');
    expect(docsSource).toContain("return 'text-amber-600 dark:text-amber-400'");
    expect(docsSource).toContain('text-emerald-600 dark:text-emerald-400');
    expect(docsSource).toContain('shrink-0 text-xs font-medium');
    expect(docsSource).not.toContain('<QuietStatusText');
  });

  it('keeps public shared documents outside the authenticated workspace header contract', () => {
    expect(source('../../../pages/docs/SharedDocumentView.tsx')).not.toContain('workspaceSidebarSafeInsetClassName');
  });
});
