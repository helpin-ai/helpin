import { test, expect } from '@playwright/test';
async function setup(page: import('@playwright/test').Page) {
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname;
    let data: unknown = [];
    if (path.endsWith('/me')) data = { membership: { role: 'owner', status: 'active' }, permissions: ['crm.read', 'crm.edit', 'settings.read', 'workspace.read', 'module_access.manage'], team_memberships: [] };
    if (path.endsWith('/email/accounts')) data = [{ id: 'mailbox', member_id: 'owner', email_address: 'owner@example.com', status: 'connected', provider: 'gmail', signature: 'My signature' }];
    if (path.endsWith('/sync-settings')) data = { historical_sync_days: 90, filter_mode: 'blocklist', filter_patterns: [], internal_exclusion: 'none', include_private_meetings: false, include_solo_meetings: false, record_creation_mode: 'selective', blocked_record_prefixes: [] };
    return route.fulfill({ json: data });
  });
}
for (const mode of ['light', 'dark', 'narrow']) test(`settings directory and search ${mode}`, async ({ page }) => {
  await setup(page);
  if (mode === 'narrow') await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/e2e/crm/harness/settings-discovery.html${mode === 'dark' ? '?dark' : ''}`);
  await expect(page.getByRole('heading', { name: 'Settings', exact: true })).toBeVisible();
  await expect(page.getByRole('region', { name: 'Integrations', exact: true })).toBeVisible();
  await page.screenshot({ path: `/tmp/settings-home-${mode}.png`, fullPage: true });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  const search = page.getByRole('searchbox', { name: 'Search settings', exact: true });
  if (mode !== 'narrow') { await page.getByRole('link', { name: 'Search settings', exact: true }).click(); await expect(search).toBeFocused(); }
  await search.fill('gmail signature');
  await expect(page.getByRole('link', { name: /Email signature/ })).toHaveAttribute('href', '/w/settings-test/settings/crm-email#email-signature');
  await search.press('Enter');
  await expect(page.getByRole('textbox', { name: /Signature for/ })).toBeVisible();
  await expect(page.getByRole('textbox', { name: /Signature for/ })).toHaveValue('My signature');
  await page.goBack();
  await expect(page.getByRole('region', { name: 'Recently visited' })).toContainText('Email');
  await page.getByRole('searchbox', { name: 'Search settings', exact: true }).fill('nothing-matches-xyz');
  await expect(page.getByRole('status')).toContainText('No matching settings');
});
