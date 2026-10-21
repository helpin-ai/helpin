import { expect, test } from '@playwright/test';
import { CONVERSATION_ID, installSupportAppMocks, WORKSPACE_SLUG } from '../fixtures/supportE2E';

test('assignee load failures stay distinct from empty results and can be retried', async ({ page }) => {
  test.setTimeout(180_000);
  await page.setViewportSize({ width: 1440, height: 900 });
  await installSupportAppMocks(page);
  let failed = true;
  await page.route(
    url => url.pathname === `/api/support/inbox/conversations/${CONVERSATION_ID}/assignees`,
    route => failed
      ? route.fulfill({ status: 400, json: { error: 'Unable to load assignees' } })
      : route.fulfill({ json: [] }),
  );
  await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`);
  const error = page.getByText('Unable to load teammates.', { exact: true });
  const empty = page.getByText('No active workspace members to assign.');
  await expect(page.getByText('Assignee', { exact: true })).toBeVisible({ timeout: 120_000 });
  await expect(error).toBeVisible();
  await expect(empty).toHaveCount(0);
  failed = false;
  await page.getByRole('button', { name: 'Retry teammates' }).click();
  await expect(error).toHaveCount(0);
  await expect(empty).toBeVisible();
});
