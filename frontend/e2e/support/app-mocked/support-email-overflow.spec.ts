import { expect, test } from '@playwright/test';
import { CONVERSATION_ID, installSupportAppMocks, SUPPORT_MESSAGES, WORKSPACE_SLUG } from '../fixtures/supportE2E';

for (const width of [390, 1440]) {
  test(`wide emails scroll inside their bubble (${width}px)`, async ({ page }) => {
    test.setTimeout(120_000);
    await page.setViewportSize({ width, height: 950 });
    await installSupportAppMocks(page);
    const messages = [
      {
        ...SUPPORT_MESSAGES[0], id: 'wide-email', via_channel: 'email', content: 'Wide email',
        html_body: `<style>html::-webkit-scrollbar { height: 16px; }</style>
          <table style="min-width:900px"><tr><td>Left column</td><td style="text-align:right">Rightmost column</td></tr></table>
          <p id="last-line">Last line of the email.</p>
          <div data-helpin-quote="true"><table style="min-width:1300px;height:160px"><tr><td>Quoted reply</td></tr></table></div>`,
      },
      {
        ...SUPPORT_MESSAGES[0], id: 'normal-email', via_channel: 'email', content: 'Normal email',
        created_at: '2026-03-27T19:57:00Z', html_body: '<p>A normal email that wraps naturally within the available space.</p>',
      },
    ];
    await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}/message-pages?**`, route =>
      route.fulfill({ json: { data: messages, has_more: false } }));
    await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`);
    const row = page.locator('[data-support-message-id="wide-email"]:visible');
    await expect(row).toBeVisible({ timeout: 90_000 });
    const iframe = row.locator('iframe');
    const body = iframe.contentFrame().locator('body');
    await expect(body.getByText('Rightmost column')).toBeVisible();
    await iframe.hover();
    await page.mouse.wheel(2000, 0);
    await expect.poll(() => body.evaluate(el => el.ownerDocument.scrollingElement!.scrollLeft)).toBeGreaterThan(0);
    expect(await body.evaluate(el => {
      const doc = el.ownerDocument;
      return doc.querySelector('table')!.getBoundingClientRect().right <= doc.documentElement.clientWidth;
    })).toBe(true);

    const collapsedHeight = await iframe.evaluate(el => el.clientHeight);
    await row.getByRole('button', { name: 'Show quoted content' }).click();
    await expect(body.getByText('Quoted reply')).toBeVisible();
    await expect.poll(() => iframe.evaluate(el => el.clientHeight)).toBeGreaterThan(collapsedHeight);
    await row.getByRole('button', { name: 'Hide quoted content' }).click();
    await expect(body.getByText('Quoted reply')).toBeHidden();
    await expect.poll(() => iframe.evaluate(el => el.clientHeight)).toBe(collapsedHeight);
    expect(await body.evaluate(el => {
      const doc = el.ownerDocument;
      return doc.querySelector('#last-line')!.getBoundingClientRect().bottom <= doc.documentElement.clientHeight;
    })).toBe(true);

    const normal = page.locator('[data-support-message-id="normal-email"]:visible iframe').contentFrame().locator('body');
    expect(await normal.evaluate(el => el.ownerDocument.documentElement.scrollWidth <= el.ownerDocument.documentElement.clientWidth)).toBe(true);
    const thread = page.locator('[data-support-message-list]:visible');
    expect(await thread.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
    await row.hover();
    const rowBounds = await row.boundingBox();
    const actionBounds = await row.getByRole('button', { name: 'Message actions' }).boundingBox();
    expect(actionBounds!.x + actionBounds!.width).toBeLessThanOrEqual(rowBounds!.x + rowBounds!.width);
  });
}
