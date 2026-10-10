import { expect, test, type Page } from '@playwright/test';
import { buildConversation, CONVERSATION_ID, installSupportAppMocks, WORKSPACE_SLUG } from '../fixtures/supportE2E';

async function setup(page: Page, hasMore = false, canEdit = true) {
  await installSupportAppMocks(page);
  if (!canEdit) {
    await page.route((url) => url.pathname === '/api/workspaces/ws-1/me', (route) => route.fulfill({ json: {
      workspace_id: 'ws-1', modules: ['support'], membership: { role: 'viewer', status: 'active' },
      permissions: ['workspace.read', 'support.read', 'settings.read', 'ws.connect'], team_memberships: [],
    } }));
  }
  await page.route((url) => url.pathname === '/api/support/inbox/mailboxes/scopes', (route) => route.fulfill({ json: {
    shared_inbox: { id: 'shared', name: 'Shared Inbox' }, mailboxes: [{ id: 'team-inbox', name: 'Billing' }],
  } }));
  const conversations = [buildConversation(), { ...buildConversation(2), id: 'conv-2', customer_name: 'Second Visitor', subject: 'Second conversation' }];
  await page.route((url) => url.pathname === '/api/support/inbox/conversations', (route) => {
    const url = new URL(route.request().url());
    const currentPage = Number(url.searchParams.get('page') ?? 1);
    return route.fulfill({ json: {
      data: currentPage === 1 ? conversations : [{ ...buildConversation(), id: 'conv-3', customer_name: 'Third Visitor' }],
      page: currentPage, per_page: Number(url.searchParams.get('per_page') ?? 50), total_pages: hasMore ? 2 : 1, total: hasMore ? 3 : 2,
    } });
  });
  await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`);
  await expect(page.locator('[data-conversation-id="conv-2"]')).toBeVisible({ timeout: 60_000 });
}

for (const theme of ['light', 'dark']) {
  test(`bulk selection uses the list header and leaves the open thread unchanged (${theme})`, async ({ page }, testInfo) => {
    test.setTimeout(90_000);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.addInitScript((theme) => localStorage.setItem('theme', theme), theme);
    await setup(page);
    const row = page.locator('[data-conversation-id="conv-2"]');
    const avatar = row.locator('[data-slot="avatar"]');
    await page.mouse.move(1400, 850);
    await expect(avatar).toBeVisible();
    await row.hover();
    await expect(avatar).toBeVisible();
    await row.getByRole('checkbox').click();
    await expect(page).toHaveURL(new RegExp(`/support/${CONVERSATION_ID}$`));
    await expect(page.getByText('1 selected', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'New conversation', exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'More bulk actions' })).toBeVisible();
    await expect(row.getByRole('button', { name: /Open actions/ })).toHaveCount(0);
    await page.locator('[data-support-conversation-scroll]').locator('..').screenshot({ path: testInfo.outputPath(`bulk-selection-${theme}.png`) });
    await page.getByRole('checkbox', { name: 'Select all 2 loaded conversations' }).click();
    await expect(page.getByText('2 selected', { exact: true })).toBeVisible();
    await expect(page.locator('[data-bulk-selected="true"]')).toHaveCount(2);
    await page.getByRole('button', { name: 'Clear selection', exact: true }).click();
    await expect(page.getByRole('button', { name: 'New conversation', exact: true })).toBeVisible();
    await expect(page.locator('[data-bulk-selected="true"]')).toHaveCount(0);
  });
}

test('selects unloaded matching conversations and keeps only failed actions selected', async ({ page }) => {
  test.setTimeout(90_000);
  await page.setViewportSize({ width: 1440, height: 900 });
  await setup(page, true);
  const updated: string[] = [];
  await page.route((url) => /\/api\/support\/inbox\/conversations\/[^/]+\/status$/.test(url.pathname), async (route) => {
    const id = new URL(route.request().url()).pathname.split('/').at(-2)!;
    updated.push(id);
    if (id === 'conv-2') return route.fulfill({ status: 403, json: { error: 'No longer accessible' } });
    return route.fulfill({ json: { ...buildConversation(), id, status: 'resolved' } });
  });
  await page.locator('[data-conversation-id="conv-2"]').hover();
  await page.locator('[data-conversation-id="conv-2"]').getByRole('checkbox').click();
  await page.getByRole('button', { name: 'Select all 3 matching conversations' }).click();
  await expect(page.getByText('3 selected', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Resolve conversations', exact: true }).click();
  await expect.poll(() => updated.slice().sort()).toEqual(['conv-1', 'conv-2', 'conv-3']);
  await expect(page.getByText('1 selected', { exact: true })).toBeVisible();
  await expect(page.locator('[data-bulk-selected="true"]')).toHaveCount(1);
  await expect(page.locator('[data-conversation-id="conv-2"]').getByRole('checkbox')).toBeChecked();
  await expect(page.getByText('1 conversation could not be updated')).toBeVisible();
  await page.getByRole('button', { name: 'Clear selection', exact: true }).focus();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('button', { name: 'More bulk actions' })).toHaveCount(0);
});

test('a phone can select without opening the conversation sheet', async ({ page }) => {
  test.setTimeout(90_000);
  await page.setViewportSize({ width: 390, height: 844 });
  await installSupportAppMocks(page);
  await page.goto(`/w/${WORKSPACE_SLUG}/support`);
  const row = page.locator(`[data-conversation-id="${CONVERSATION_ID}"]`);
  await expect(row).toBeVisible({ timeout: 60_000 });
  await row.getByRole('checkbox').focus();
  await page.keyboard.press('Space');
  await expect(page.getByText('1 selected', { exact: true })).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`/support$`));
  const header = page.locator('[data-slot="support-inbox-panel-header"]').first();
  const bounds = await header.boundingBox();
  const clearBounds = await header.getByRole('button', { name: 'Clear selection', exact: true }).boundingBox();
  expect(clearBounds!.x + clearBounds!.width).toBeLessThanOrEqual(bounds!.x + bounds!.width);
});

test('bulk deletion confirms the count and sends no changes when cancelled', async ({ page }) => {
  test.setTimeout(90_000);
  await setup(page);
  const deleted: string[] = [];
  await page.route((url) => /\/api\/support\/inbox\/conversations\/[^/]+$/.test(url.pathname), (route) => {
    if (route.request().method() !== 'DELETE') return route.fallback();
    deleted.push(new URL(route.request().url()).pathname.split('/').at(-1)!);
    return route.fulfill({ json: { message: 'Deleted' } });
  });
  await page.locator('[data-conversation-id="conv-2"]').hover();
  await page.locator('[data-conversation-id="conv-2"]').getByRole('checkbox').click();
  await page.getByRole('checkbox', { name: 'Select all 2 loaded conversations' }).click();
  await page.getByRole('button', { name: 'More bulk actions' }).click();
  await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();
  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('Delete 2 conversations?');
  expect(deleted).toEqual([]);
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(page.getByText('2 selected', { exact: true })).toBeVisible();
  expect(deleted).toEqual([]);
  await page.getByRole('button', { name: 'More bulk actions' }).click();
  await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();
  await dialog.getByRole('button', { name: 'Delete', exact: true }).click();
  await expect.poll(() => deleted.slice().sort()).toEqual(['conv-1', 'conv-2']);
  await expect(page.getByRole('button', { name: 'More bulk actions' })).toHaveCount(0);
});

test('viewers only get personal read actions and Escape closes the menu before clearing selection', async ({ page }) => {
  test.setTimeout(90_000);
  await setup(page, false, false);
  await page.locator('[data-conversation-id="conv-2"]').hover();
  await page.locator('[data-conversation-id="conv-2"]').getByRole('checkbox').click();
  await expect(page.getByRole('button', { name: 'Resolve conversations' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Move conversations to inbox' })).toHaveCount(0);
  await page.getByRole('button', { name: 'More bulk actions' }).click();
  await expect(page.getByRole('menuitem')).toHaveCount(2);
  await expect(page.getByRole('menuitem', { name: 'Mark as read', exact: true })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Mark as unread', exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('menu')).toHaveCount(0);
  await expect(page.getByText('1 selected', { exact: true })).toBeVisible();
  await page.locator('[data-conversation-id="conv-2"]').getByRole('checkbox').focus();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('button', { name: 'More bulk actions' })).toHaveCount(0);
});

test('moves selected conversations through the inbox selector', async ({ page }) => {
  test.setTimeout(90_000);
  await setup(page);
  const moved: Array<{ id: string; mailbox_id: string }> = [];
  await page.route((url) => /\/api\/support\/inbox\/conversations\/[^/]+\/move$/.test(url.pathname), (route) => {
    const id = new URL(route.request().url()).pathname.split('/').at(-2)!;
    moved.push({ id, ...route.request().postDataJSON() });
    return route.fulfill({ json: { ...buildConversation(), id, mailbox_id: 'team-inbox' } });
  });
  await page.locator('[data-conversation-id="conv-2"]').hover();
  await page.locator('[data-conversation-id="conv-2"]').getByRole('checkbox').click();
  await page.getByRole('button', { name: 'Move conversations to inbox' }).click();
  await page.getByRole('option', { name: 'Billing', exact: true }).click();
  await expect.poll(() => moved).toEqual([{ id: 'conv-2', mailbox_id: 'team-inbox' }]);
  await expect(page.getByRole('button', { name: 'More bulk actions' })).toHaveCount(0);
});
