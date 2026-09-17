import { test, expect } from '@playwright/test';
for (const mode of ['light', 'dark'] as const) {
 test(`edits, saves, and shows condition outcomes on mobile in ${mode} mode`, async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/e2e/automation/harness/semantic-condition.html');
  if (mode === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
  const input = page.getByLabel('Semantic condition (optional)');
  await expect(input).toHaveValue('The release concerns authentication');
  await input.focus(); await expect(input).toBeFocused();
  await input.fill('The release fixes login failures');
  await page.keyboard.press('Tab'); await expect(page.getByRole('button', { name: 'Save', exact: true })).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(page.locator('output')).toContainText('The release fixes login failures');
  await expect(page.getByText('Condition was uncertain — action skipped')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: `/tmp/helpin-jev-flow-${mode}.png`, fullPage: true });
  await page.getByRole('button', { name: 'Switch trigger' }).click();
  await expect(page.getByText('Remove this condition before using a schedule.')).toBeVisible();
  await input.fill(''); await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.locator('output')).not.toContainText('semantic_condition');
  await expect(input).toBeDisabled();
 });
}
