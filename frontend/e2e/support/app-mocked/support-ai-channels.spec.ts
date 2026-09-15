import { expect, test } from '@playwright/test';
import { installSupportAppMocks, SUPPORT_INSTALLATION, WORKSPACE_SLUG } from '../fixtures/supportE2E';

test('AI reply channels save and survive reload on desktop and narrow screens', async ({ page }) => {
  test.setTimeout(120_000);
  await installSupportAppMocks(page);
  let settings: Record<string, unknown> = { ...SUPPORT_INSTALLATION.settings, ai_response_mode: 'ai_first' };
  const writes: Record<string, unknown>[] = [];
  await page.route('**/api/support/inbox/installations?**', async (route) => {
    if (route.request().method() === 'PATCH') {
      const patch = route.request().postDataJSON();
      writes.push(patch);
      settings = { ...settings, ...patch };
    }
    await route.fulfill({ json: { ...SUPPORT_INSTALLATION, settings } });
  });
  await page.goto(`/w/${WORKSPACE_SLUG}/settings/support-ai-assistant`);
  const channels = page.getByRole('combobox', { name: 'Reply channels' });
  await expect(channels).toHaveText('Chat');
  for (const [label, value] of [['Email', 'email'], ['Both', 'both'], ['Chat', 'chat']] as const) {
    await channels.click();
    await page.getByRole('option', { name: label, exact: true }).click();
    await expect.poll(() => writes.at(-1)?.ai_reply_channels).toBe(value);
    await page.reload();
    await expect(channels).toHaveText(label);
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(channels).toBeVisible();
  await channels.focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('option', { name: 'Email', exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(channels).toBeFocused();
  await page.screenshot({ path: '/tmp/helpin-ai-channels-mobile.png', fullPage: true });
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.screenshot({ path: '/tmp/helpin-ai-channels-desktop.png', fullPage: true });
});
