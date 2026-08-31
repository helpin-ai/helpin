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
    ['space and collection pages', '../../../pages/docs/spaceDetail/SpaceNodeHeader.tsx'],
  ])('uses QuietPageHeader for %s', (_label, path) => {
    expect(source(path)).toContain('<QuietPageHeader');
  });

  it.each([
    ['Teams detail', '../../settings/TeamsTab.tsx'],
  ])('keeps the specialized %s toolbar clear of the collapsed sidebar opener', (_label, path) => {
    expect(source(path)).toContain('workspaceSidebarSafeInsetClassName');
  });

  it.each([
    ['Docs editor', '../../../pages/docs/DocsDocumentDetail.tsx'],
    ['Meeting detail', '../../../pages/crm/MeetingDetail.tsx'],
    ['Contact detail', '../../crm/contact-detail/ContactHeader.tsx'],
    ['Company detail', '../../../pages/crm/CompanyDetail.tsx'],
  ])('uses the shared detail header contract for %s', (_label, path) => {
    expect(source(path)).toContain('<QuietDetailHeader');
    expect(source(path)).toContain('<QuietBreadcrumbs');
  });

  it('renders the private Docs title in the header instead of repeating it in the editor', () => {
    const docsSource = source('../../../pages/docs/DocsDocumentDetail.tsx');
    expect(docsSource).toContain('<QuietTitleTextarea');
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
