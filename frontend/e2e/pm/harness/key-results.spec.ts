import { test, expect, type Page } from '@playwright/test';

async function setup(page: Page, query = '') {
  const writes: Record<string, unknown>[] = [];
  await page.route('**/api/pm/key-results/**', async route => {
    const headers = { 'access-control-allow-origin': 'http://127.0.0.1:5194', 'access-control-allow-credentials': 'true', 'access-control-allow-headers': 'content-type, authorization', 'access-control-allow-methods': 'PUT, OPTIONS' };
    if (route.request().method() === 'OPTIONS') return route.fulfill({ status: 204, headers });
    const patch = route.request().postDataJSON();
    writes.push(patch);
    const id = new URL(route.request().url()).pathname.split('/').at(-1);
    const current = patch.current_value ?? 35;
    return route.fulfill({ headers, contentType: 'application/json', body: JSON.stringify({ id, objective_id: 'objective', name: 'Increase trial-to-paid conversion', result_type: 'percent', initial_value: 0, current_value: current, target_value: 100, progress: current, position: 0, created_at: '2026-09-01T00:00:00Z', updated_at: new Date().toISOString(), ...patch }) });
  });
  await page.goto(`/e2e/pm/harness/key-results.html${query}`);
  await expect(page.getByRole('region', { name: 'Key results' })).toBeVisible();
  return writes;
}

test('supports inline values and an explicit full editor without losing the latest value', async ({ page }) => {
  const writes = await setup(page);
  const current = page.getByRole('spinbutton', { name: 'Current value for Increase trial-to-paid conversion' });
  await current.fill('42');
  await page.getByRole('button', { name: 'Edit key result Increase trial-to-paid conversion' }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByLabel('Current value', { exact: true })).toHaveValue('42');
  await expect(dialog.getByLabel('Target value')).toHaveValue('100');
  await dialog.getByLabel('Name', { exact: true }).fill('Increase customer activation');
  await page.screenshot({ path: '/tmp/helpin-key-results-editor.png' });
  await dialog.getByRole('button', { name: 'Save changes' }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText('Increase customer activation', { exact: true })).toBeVisible();
  expect(writes.at(-1)).toMatchObject({ name: 'Increase customer activation', current_value: 42, target_value: 100 });
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
  await expect(page.getByText('Not completed', { exact: true })).toBeVisible();
  await expect(page.getByRole('progressbar')).toHaveCount(3);
});
