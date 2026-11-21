import { expect, test, type Page } from '@playwright/test';
import { installSupportAppMocks, WORKSPACE_ID, WORKSPACE_SLUG } from '../fixtures/supportE2E';

test.setTimeout(120_000);

const settings = `/w/${WORKSPACE_SLUG}/settings`;

async function workspaceWithTools(page: Page, modules: string[], manage: boolean, supportAdmin = false) {
  await installSupportAppMocks(page);
  await page.route(`**/api/workspaces/${WORKSPACE_ID}/me`, route => route.fulfill({ json: {
    workspace_id: WORKSPACE_ID, modules, membership: { id: 'member', role: manage ? 'admin' : 'member' },
    permissions: ['workspace.read', 'settings.read', ...(manage ? ['settings.manage'] : []), ...(supportAdmin ? ['support.admin'] : [])], team_memberships: [],
  } }));
  const requests = { slack: 0, mcp: 0, telegram: 0 };
  await page.route('**/api/support/channels?**', route => {
    requests.telegram++;
    return route.fulfill({ json: [] });
  });
  await page.route('**/api/slack/?**', route => {
    requests.slack++;
    return route.fulfill({ json: { configured: true, connected: false, routes: [] } });
  });
  await page.route('**/api/external-mcp/providers?**', route => {
    requests.mcp++;
    return route.fulfill({ json: { enabled: true, providers: [] } });
  });
  await page.route('**/api/external-mcp/servers?**', route => route.fulfill({ json: { servers: [] } }));
  return requests;
}

test('a settings admin can manage Slack without Agents and gets one Channels shortcut', async ({ page }) => {
  const requests = await workspaceWithTools(page, ['support'], true);
  await page.goto(settings + '/support-channels', { waitUntil: 'domcontentloaded' });
  const shortcut = page.getByRole('link', { name: 'Manage Slack' });
  await expect(shortcut).toBeVisible({ timeout: 60_000 });
  await expect(page.getByRole('button', { name: 'Connect Slack' })).toHaveCount(0);
  await expect(page.getByRole('link', { name: 'Manage Telegram' })).toHaveCount(0);
  await shortcut.click();
  await expect(page.getByRole('button', { name: 'Connect Slack' })).toBeEnabled({ timeout: 60_000 });
  await expect(page.getByRole('tab', { name: 'MCP servers' })).toHaveCount(0);
  expect(requests.mcp).toBe(0);
  expect(requests.telegram).toBe(0);
});

test('a Support admin can reach Telegram from Channels without workspace settings management', async ({ page }) => {
  const requests = await workspaceWithTools(page, ['support'], false, true);
  await page.goto(settings + '/support-channels');
  await page.getByRole('link', { name: 'Manage Telegram' }).click();
  await expect(page).toHaveURL(/external-tools\?tab=apps#telegram$/);
  await page.getByRole('button', { name: 'Connect bot' }).click();
  await expect(page.getByRole('dialog', { name: 'Connect a Telegram bot' })).toBeVisible();
  expect(requests.telegram).toBeGreaterThan(0);
  expect(requests.slack).toBe(0);
  expect(requests.mcp).toBe(0);
});

test('the Channels shortcut lands on Telegram even below a long list of Slack channels', async ({ page }) => {
  await workspaceWithTools(page, ['support'], true, true);
  const routes = Array.from({ length: 30 }, (_, i) => ({ channel_id: 'C' + i, channel_name: 'support-' + i, purpose: 'support' }));
  await page.route('**/api/slack/?**', route => route.fulfill({ json: { connected: true, workspace_name: 'Northstar', routes } }));
  await page.goto(settings + '/support-channels');
  await page.getByRole('link', { name: 'Manage Telegram' }).click();
  await expect(page.getByRole('button', { name: /#support-29/ })).toBeAttached();
  await expect(page.getByRole('button', { name: 'Connect bot' })).toBeInViewport();
});

test('a PM-only admin who bookmarked Channels reaches Apps without loading MCP', async ({ page }) => {
  const requests = await workspaceWithTools(page, ['pm'], true);
  await page.goto(settings + '/support-channels', { waitUntil: 'domcontentloaded' });
  await expect(page).toHaveURL(/\/external-tools\?tab=apps$/);
  await expect(page.getByRole('button', { name: 'Connect Slack' })).toBeEnabled();
  await expect(page.getByRole('tab', { name: 'MCP servers' })).toHaveCount(0);
  expect(requests.mcp).toBe(0);
  expect(requests.telegram).toBe(0);
});

test('existing MCP bookmarks open the read-only MCP tab without loading Slack', async ({ page }) => {
  const requests = await workspaceWithTools(page, ['agents'], false);
  await page.goto(settings + '/external-mcp', { waitUntil: 'domcontentloaded' });
  await expect(page).toHaveURL(/\/settings\/external-tools\?tab=mcp$/);
  await expect(page.getByText('Connections are read-only')).toBeVisible({ timeout: 60_000 });
  await expect(page.getByRole('tab', { name: 'Apps' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Add server', exact: true })).toHaveCount(0);
  expect(requests.slack).toBe(0);
});

test('switching tools and going back restores the tab through normal navigation', async ({ page }) => {
  await workspaceWithTools(page, ['pm', 'agents'], true);
  await page.goto(settings + '/external-tools', { waitUntil: 'domcontentloaded' });
  await expect(page.getByRole('tab', { name: 'Apps' })).toHaveAttribute('aria-selected', 'true', { timeout: 60_000 });
  await page.getByRole('tab', { name: 'MCP servers' }).click();
  await expect(page.getByRole('button', { name: 'Add server', exact: true })).toBeEnabled();
  await expect(page).toHaveURL(/\?tab=mcp$/);
  await page.goBack();
  await expect(page.getByRole('button', { name: 'Connect Slack' })).toBeEnabled();
});
