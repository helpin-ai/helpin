import { test, expect, type Page } from '@playwright/test';

async function setup(page: Page, query = '') {
  const writes: Record<string, unknown>[] = [];
  let current = 35;
  await page.route('**/api/pm/key-results/**', async route => {
    const headers = { 'access-control-allow-origin': 'http://127.0.0.1:5194', 'access-control-allow-credentials': 'true', 'access-control-allow-headers': 'content-type, authorization', 'access-control-allow-methods': 'GET, PUT, OPTIONS' };
    if (route.request().method() === 'OPTIONS') return route.fulfill({ status: 204, headers });
    if (route.request().method() === 'GET') {
      const resultId = new URL(route.request().url()).pathname.split('/').at(-2);
      const entries = resultId === 'activation' ? [current, 30, 20].map((value, index) => ({ activity: { id: String(index), workspace_id: 'ws', entity_type: 'key_result', entity_id: resultId, actor_id: 'aro oj', action: 'updated', field_name: 'current_value', new_value: String(value), metadata: { result_type: 'percent' }, created_at: '2026-09-25T10:00:00Z' }, actor: { id: 'aro oj', full_name: 'Arooj' } })) : [];
      return route.fulfill({ headers, contentType: 'application/json', body: JSON.stringify({ data: entries, total: entries.length, page: 1, per_page: 20, total_pages: entries.length ? 1 : 0 }) });
    }
    const patch = route.request().postDataJSON();
    writes.push(patch);
    const id = new URL(route.request().url()).pathname.split('/').at(-1);
    current = patch.current_value ?? current;
    return route.fulfill({ headers, contentType: 'application/json', body: JSON.stringify({ id, objective_id: 'objective', name: 'Increase trial-to-paid conversion', result_type: 'percent', initial_value: 0, current_value: current, target_value: 100, progress: current, position: 0, created_at: '2026-09-01T00:00:00Z', updated_at: new Date().toISOString(), ...patch }) });
  });
  await page.goto(`/e2e/pm/harness/key-results.html${query}`);
  await expect(page.getByRole('region', { name: 'Key results' })).toBeVisible();
  return writes;
}

test('logs values in a dialog and edits settings without overwriting the current value', async ({ page }) => {
  const writes = await setup(page);
  await expect(page.getByRole('spinbutton')).toHaveCount(0);
  await expect(page.getByRole('progressbar')).toHaveCount(0);
  await page.getByText('Increase trial-to-paid conversion', { exact: true }).hover();
  await page.getByRole('button', { name: 'Log result for Increase trial-to-paid conversion' }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByRole('radio')).toHaveCount(0);
  await dialog.getByLabel('New current value').fill('42');
  await dialog.getByRole('button', { name: 'Log result', exact: true }).click();
  await expect(dialog).toHaveCount(0);
  expect(writes.at(-1)).toEqual({ current_value: 42 });
  await page.getByText('Increase trial-to-paid conversion', { exact: true }).hover();
  await page.getByRole('button', { name: 'Edit key result Increase trial-to-paid conversion' }).click();
  await expect(dialog.getByLabel('Current value', { exact: true })).toHaveCount(0);
  await expect(dialog.getByLabel('Target value')).toHaveValue('100');
  await dialog.getByLabel('Name', { exact: true }).fill('Increase customer activation');
  await page.screenshot({ path: '/tmp/helpin-key-results-editor.png' });
  await dialog.getByRole('button', { name: 'Save changes' }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText('Increase customer activation', { exact: true })).toBeVisible();
  expect(writes.at(-1)).toMatchObject({ name: 'Increase customer activation', target_value: 100 });
  expect(writes.at(-1)).not.toHaveProperty('current_value');
});

test('keeps history collapsed and exposes earlier updates without repeating the latest', async ({ page }) => {
  await setup(page);
  await expect(page.getByText('Arooj', { exact: true })).toHaveCount(1);
  await page.getByRole('button', { name: 'Show 2 earlier updates' }).click();
  await expect(page.getByRole('list', { name: 'Earlier updates for Increase trial-to-paid conversion' }).getByRole('listitem')).toHaveCount(2);
  await expect(page.getByText('Arooj', { exact: true })).toHaveCount(3);
  await page.screenshot({ path: '/tmp/helpin-key-results-history.png', fullPage: true });
  await page.getByRole('button', { name: 'Hide earlier updates' }).click();
  await expect(page.getByText('Arooj', { exact: true })).toHaveCount(1);
});

test('hover actions use no reserved column and do not move the values', async ({ page }) => {
  await setup(page);
  const row = page.getByText('Increase trial-to-paid conversion', { exact: true }).locator('..').locator('..');
  const values = row.locator('dl');
  const before = await values.boundingBox();
  const bounds = await row.boundingBox();
  expect(Math.abs(before!.x + before!.width - bounds!.x - bounds!.width)).toBeLessThan(2);
  await row.hover();
  const after = await values.boundingBox();
  expect(after).toEqual(before);
  const log = row.getByRole('button', { name: 'Log result for Increase trial-to-paid conversion' });
  await expect(log).toBeVisible();
  const action = await log.boundingBox();
  expect(action!.y).toBeGreaterThanOrEqual(after!.y + after!.height);
  expect(action!.y + action!.height).toBeLessThanOrEqual(bounds!.y + bounds!.height);
  await page.screenshot({ path: '/tmp/helpin-key-results-desktop.png', fullPage: true });
});

for (const theme of ['light', 'dark']) {
  test(`keeps values and editing visible on narrow screens in ${theme} mode`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await setup(page, `?theme=${theme}`);
    await expect(page.getByText('Reduce the average first response time for customers contacting our support team', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: /Edit key result/ })).toHaveCount(3);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: `/tmp/helpin-key-results-mobile-${theme}.png`, fullPage: true });
    await page.getByRole('button', { name: 'Edit key result Increase trial-to-paid conversion' }).click();
    await expect(page.getByRole('dialog').getByRole('button', { name: 'Save changes' })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}

test('read-only results retain values without editing affordances', async ({ page }) => {
  await setup(page, '?readonly');
  await expect(page.getByRole('button', { name: /Edit key result/ })).toHaveCount(0);
  await expect(page.getByRole('spinbutton')).toHaveCount(0);
  await expect(page.getByText('Incomplete', { exact: true })).toBeVisible();
  await expect(page.getByRole('progressbar')).toHaveCount(0);
  await expect(page.getByRole('button', { name: /Log result/ })).toHaveCount(0);
});
