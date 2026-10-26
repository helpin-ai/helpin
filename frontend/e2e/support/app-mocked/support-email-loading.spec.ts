import { expect, test } from '@playwright/test';
import { CONVERSATION_ID, installSupportAppMocks, SUPPORT_MESSAGES, WORKSPACE_SLUG } from '../fixtures/supportE2E';

test('email text and quote controls render while a signature image is still loading', async ({ page }) => {
  test.setTimeout(120_000);
  await installSupportAppMocks(page);
  let releaseImage!: () => void;
  const imagePending = new Promise<void>(resolve => { releaseImage = resolve; });
  await page.route('https://email-assets.example/signature.png', async route => {
    await imagePending;
    await route.abort();
  });
  await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}/message-pages?**`, route =>
    route.fulfill({ json: { data: [{
      ...SUPPORT_MESSAGES[0], id: 'loading-email', via_channel: 'email',
      content: 'Hello!\nHere is my complete question.\nThank you.',
      html_body: '<p>Hello!</p><p>Here is my complete question.</p><p id="last-line">Thank you.</p><img src="https://email-assets.example/signature.png"><div data-helpin-quote="true">Earlier message</div>',
    }], has_more: false } }));
  try {
    await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`, { waitUntil: 'domcontentloaded' });
    const row = page.locator('[data-support-message-id="loading-email"]:visible');
    await expect(row).toBeVisible({ timeout: 90_000 });
    const iframe = row.locator('iframe');
    const body = iframe.contentFrame().locator('body');
    await expect(body.locator('#last-line')).toHaveText('Thank you.');
    await expect.poll(() => iframe.evaluate(el => el.clientHeight)).toBeGreaterThan(80);
    await expect(row.getByRole('button', { name: 'Show quoted content' })).toBeVisible();
    await expect(body.getByText('Earlier message')).toBeHidden();
    await row.getByRole('button', { name: 'Show quoted content' }).click();
    await expect(body.getByText('Earlier message')).toBeVisible();
    releaseImage();
    await expect.poll(() => iframe.evaluate(el => (el as HTMLIFrameElement).contentDocument?.readyState)).toBe('complete');
    await expect(body.getByText('Earlier message')).toBeVisible();
    await row.getByRole('button', { name: 'Hide quoted content' }).click();
    await expect(body.getByText('Earlier message')).toBeHidden();
  } finally {
    releaseImage();
  }
});
