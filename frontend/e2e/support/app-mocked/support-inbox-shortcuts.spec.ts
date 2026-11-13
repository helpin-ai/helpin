import { expect, test, type Page } from '@playwright/test';
import { buildConversation, CONVERSATION_ID, installSupportAppMocks, WORKSPACE_SLUG } from '../fixtures/supportE2E';

// Enough teammates that the assignee menu keeps its search box.
const ASSIGNEES = ['Alice Agent', 'Bob Agent', 'Charlie Agent', 'Dana Lee', 'Eli Moss', 'Fay Wong', 'Gus Hart', 'Hana Ito', 'Ivan Ruiz', 'Jo Park', 'Kim Diaz', 'Lou Grant']
  .map((name, index) => ({
    id: `member-${index}`,
    user_id: `user-${index}`,
    role: 'member',
    email: `${name.split(' ')[0].toLowerCase()}@example.com`,
    display_name: name,
    avatar_url: null,
    status: 'active',
  }));

async function openInbox(page: Page) {
  await installSupportAppMocks(page);
  await page.route((url) => url.pathname === `/api/support/inbox/conversations/${CONVERSATION_ID}/assignees`, (route) => route.fulfill({ json: ASSIGNEES }));
  const statuses: string[] = [];
  await page.route((url) => url.pathname === `/api/support/inbox/conversations/${CONVERSATION_ID}/status`, (route) => {
    const { status } = route.request().postDataJSON() as { status: string };
    statuses.push(status);
    return route.fulfill({ json: { ...buildConversation(), status } });
  });
  const assignments: unknown[] = [];
  await page.route((url) => url.pathname === `/api/support/inbox/conversations/${CONVERSATION_ID}/assign-user`, (route) => {
    assignments.push(route.request().postDataJSON());
    return route.fulfill({ json: { success: true } });
  });
  await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`);
  await expect(page.getByRole('button', { name: 'Resolve', exact: true })).toBeVisible({ timeout: 60_000 });
  return { statuses, assignments };
}

/** Moves focus out of every text field, where a key press is not typing. */
async function leaveTextFields(page: Page) {
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
}

test('a stray E or A changes nothing, Ctrl+E resolves and A opens the assignee search', async ({ page }) => {
  test.setTimeout(90_000);
  await page.setViewportSize({ width: 1440, height: 900 });
  const { statuses, assignments } = await openInbox(page);
  await expect(page.getByRole('button', { name: 'Resolve', exact: true })).toHaveAttribute('title', 'Resolve (Ctrl+E)');
  await expect(page.getByTitle('Assign (A)')).toBeVisible();

  await leaveTextFields(page);
  await page.keyboard.press('e');
  await page.keyboard.press('a');
  const search = page.getByPlaceholder('Search members...');
  await expect(search).toBeFocused();
  await page.keyboard.type('bob');
  await expect(page.getByRole('option', { name: /Bob Agent/ })).toBeVisible();
  await expect(page.getByRole('option', { name: /Alice Agent/ })).toHaveCount(0);
  await page.keyboard.press('Escape');
  await expect(search).toHaveCount(0);
  expect(statuses).toEqual([]);
  expect(assignments).toEqual([]);

  await leaveTextFields(page);
  await page.keyboard.press('Control+e');
  await expect.poll(() => statuses).toEqual(['resolved']);
});

test('the shortcut list names Command keys on Apple devices', async ({ page }) => {
  test.setTimeout(90_000);
  await page.addInitScript(() => Object.defineProperty(navigator, 'platform', { get: () => 'MacIntel' }));
  await openInbox(page);
  await expect(page.getByRole('button', { name: 'Resolve', exact: true })).toHaveAttribute('title', 'Resolve (⌘E)');
  await leaveTextFields(page);
  await page.keyboard.press('?');
  const dialog = page.getByRole('dialog', { name: 'Keyboard shortcuts' });
  await expect(dialog.getByText('⌘E', { exact: true })).toBeVisible();
  await expect(dialog.getByText('Assign', { exact: true })).toBeVisible();
});
