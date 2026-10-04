import { expect, test } from '@playwright/test';
test.setTimeout(60_000);
const path = '/e2e/automation/harness/flow-builder.html';

for (const width of [1440, 390]) {
  test(`custom flow starts with the Ask Agent input beneath its prompt at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 844 });
    await page.goto(`${path}?mode=custom`, { waitUntil: 'domcontentloaded' });
    const input = page.getByPlaceholder('Describe your flow…');
    await expect(input).toBeVisible();
    const heading = await page.getByText('What would you like to automate?', { exact: true }).boundingBox();
    const box = await input.boundingBox();
    expect(heading).not.toBeNull();
    expect(box).not.toBeNull();
    expect(box!.y).toBeGreaterThan(heading!.y + heading!.height);
    expect(box!.y - heading!.y - heading!.height).toBeLessThan(100);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width);
    await expect(page.locator('[data-composer-actions]')).toBeVisible();
    await page.screenshot({ path: `/tmp/flow-builder-intro-${width}.png` });
    await input.fill('Review new pull requests');
    await input.press('Enter');
    await expect(page.getByText('Which repository should this flow use?', { exact: true })).toBeVisible();
    const reply = await page.getByPlaceholder('Reply or ask for a change…').boundingBox();
    expect(reply!.y).toBeGreaterThan(500);
  });
}

test('editing describes the flow, reviews changes as prose, then saves after approval', async ({ page }) => {
  await page.goto(`${path}?mode=edit`, { waitUntil: 'domcontentloaded' });
  await expect(page.getByText('What would you like to change?', { exact: true })).toBeVisible();
  await expect(page.locator('strong').filter({ hasText: 'Code Reviewer' })).toBeVisible();
  await expect(page.getByText('Current flow', { exact: true })).toHaveCount(0);
  const reply = page.getByPlaceholder('What would you like to change?');
  await reply.fill('Only if the pull request concerns authentication');
  await reply.press('Enter');
  await expect(page.getByRole('button', { name: 'Yes, save changes' })).toBeVisible();
  const summary = page.getByRole('region', { name: 'Flow summary' });
  await expect(summary).toContainText('It runs only when');
  await expect(summary.locator('strong').filter({ hasText: 'main' })).toBeVisible();
  await expect(summary.getByText('When', { exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Make changes' }).click();
  await page.getByLabel('Requested changes').fill('Use develop as the target branch');
  await page.getByRole('button', { name: 'Send changes' }).click();
  await expect(summary.locator('strong').filter({ hasText: 'develop' })).toBeVisible();
  await page.getByRole('button', { name: 'Yes, save changes' }).click();
  await expect(page.getByRole('button', { name: 'Done', exact: true })).toBeVisible();
});

test('custom and template entry use the same conversation', async ({ page }) => {
  await page.goto(path, { waitUntil: 'domcontentloaded' });
  await page.getByRole('button', { name: 'New flow', exact: true }).click();
  await page.getByPlaceholder('Describe your flow…').fill('Review new pull requests');
  await page.getByPlaceholder('Describe your flow…').press('Enter');
  await expect(page.getByText('Which repository should this flow use?', { exact: true })).toBeVisible();
  const reply = page.getByPlaceholder('Reply or ask for a change…');
  await reply.fill('acme/web-app');
  await reply.press('Enter');
  await expect(page.getByRole('button', { name: 'Yes, create flow' })).toBeVisible();
  await page.getByRole('button', { name: 'Yes, create flow' }).click();
  await expect(page.getByRole('button', { name: 'Done', exact: true })).toBeVisible();
  await page.goto(`${path}?mode=template`, { waitUntil: 'domcontentloaded' });
  await expect(page.getByText('Which repository should this flow use?', { exact: true })).toBeVisible();
});

test('edit introduction fits a narrow screen', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${path}?mode=edit`, { waitUntil: 'domcontentloaded' });
  await expect(page.getByText('What would you like to change?', { exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

for (const mode of ['custom', 'template']) {
  test(`${mode} hides validation drafts until explicit final approval`, async ({ page }) => {
    await page.goto(`${path}?mode=${mode}`, { waitUntil: 'domcontentloaded' });
    if (mode === 'custom') {
      await page.getByPlaceholder('Describe your flow…').fill('Review new pull requests');
      await page.getByPlaceholder('Describe your flow…').press('Enter');
    }
    await expect(page.getByText('Which repository should this flow use?', { exact: true })).toBeVisible();
    const summary = page.getByRole('region', { name: 'Flow summary' });
    await expect(summary).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Yes, create flow' })).toHaveCount(0);
    const reply = page.getByPlaceholder('Reply or ask for a change…');
    await reply.fill('acme/web-app');
    await reply.press('Enter');
    await expect(summary).toBeVisible();
    await expect(page.getByText('Create this flow?', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Done', exact: true })).toHaveCount(0);
    await page.getByRole('button', { name: 'Yes, create flow' }).click();
    await expect(page.getByRole('button', { name: 'Done', exact: true })).toBeVisible();
  });
}
