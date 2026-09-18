import { expect, test, type Page } from '@playwright/test';

async function mocks(
  page: Page,
  options: { available?: boolean; incomingFail?: boolean; outgoingFail?: boolean } = {},
) {
  const sends: Record<string, unknown>[] = [];
  let failures = options.outgoingFail ? 1 : 0;
  let preference = { reading_language: 'en', auto_translate_incoming: true, auto_translate_outgoing: true };
  let conversation = { customer_language: 'de', translation_mode: 'inherit' };
  await page.route('**/api/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const fulfill = (json: unknown, status = 200) => route.fulfill({ json, status });
    if (path.endsWith('/translation/preference')) {
      preference = request.postDataJSON();
      return fulfill({ saved: true });
    }
    if (path.endsWith('/translation')) {
      if (request.method() === 'PUT') {
        conversation = request.postDataJSON();
        return fulfill({ saved: true });
      }
      return fulfill({
        available: options.available !== false && conversation.translation_mode !== 'off',
        unavailable_reason: 'Translation is not configured.',
        languages: { en: 'English', de: 'German', fr: 'French' },
        preference,
        conversation,
      });
    }
    if (path.endsWith('/translation/messages')) {
      if (options.incomingFail) return fulfill({ error: 'Translation unavailable' }, 503);
      return fulfill({
        id: 'translated-incoming',
        source_text: 'Hallo, ich brauche Hilfe.',
        source_language: 'de',
        target_language: preference.reading_language,
        status: 'ready',
        translated_text:
          preference.reading_language === 'fr' ? 'Bonjour, j’ai besoin d’aide.' : 'Hello, I need help.',
      });
    }
    if (path.endsWith('/conversations/conv/messages') && request.method() === 'POST') {
      const body = request.postDataJSON();
      sends.push(body);
      if (failures-- > 0) return fulfill({ error: 'Translation failed. Your reply was not sent.' }, 503);
      return fulfill({
        id: 'sent',
        workspace_id: 'ws',
        conversation_id: 'conv',
        sender_type: 'user',
        message_type: 'reply',
        content: 'Hallo, wir können helfen.',
        created_at: new Date().toISOString(),
      });
    }
    if (path.endsWith('/conversations/conv'))
      return fulfill({
        id: 'conv',
        workspace_id: 'ws',
        source: 'widget',
        anonymous_id: 'visitor',
        status: 'open',
      });
    return fulfill([]);
  });
  return sends;
}

for (const width of [1440, 390]) {
  test(`automatic translation, original toggle and normal Send (${width}px)`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const sends = await mocks(page);
    await page.goto('/e2e/support/harness/support-translation.html');
    await expect(page.getByText('Hello, I need help.', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Show original', exact: true }).click();
    await expect(page.getByText('Hallo, ich brauche Hilfe.', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Show translation', exact: true }).click();
    await expect(page.getByText('Preview translation', { exact: true })).toHaveCount(0);
    await page.screenshot({ path: `/tmp/helpin-translation-${width}-light.png`, fullPage: true });
    await page.locator('html').evaluate((el) => el.classList.add('dark'));
    await page.screenshot({ path: `/tmp/helpin-translation-${width}-dark.png`, fullPage: true });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.getByRole('button', { name: 'Send', exact: true }).click();
    await expect.poll(() => sends.length).toBe(1);
    expect(sends[0]).toMatchObject({
      content: 'Hello, we can help.',
      auto_translate: true,
      translation_target_language: 'de',
    });
    await expect(page.locator('.tiptap')).toHaveText('');
  });
}
test('failed translation retains the draft and stable retry identity', async ({ page }) => {
  const sends = await mocks(page, { outgoingFail: true, incomingFail: true });
  await page.goto('/e2e/support/harness/support-translation.html');
  await expect(page.getByText('Translation unavailable. Original shown.')).toBeVisible();
  await page.getByRole('button', { name: 'Send', exact: true }).click();
  await expect(page.getByText('Failed to send message', { exact: true })).toBeVisible();
  await expect(page.locator('.tiptap')).toHaveText('Hello, we can help.');
  await page.getByRole('button', { name: 'Send', exact: true }).click();
  await expect.poll(() => sends.length).toBe(2);
  expect(sends[1].client_message_id).toBe(sends[0].client_message_id);
});
test('missing provider preserves ordinary messaging', async ({ page }) => {
  const sends = await mocks(page, { available: false });
  await page.goto('/e2e/support/harness/support-translation.html');
  await expect(page.getByText('Translation is not configured.', { exact: true })).toBeVisible();
  await expect(page.getByLabel('Auto-translate replies')).toHaveCount(0);
  await page.getByRole('button', { name: 'Send', exact: true }).click();
  await expect.poll(() => sends.length).toBe(1);
  expect(sends[0].auto_translate).toBeUndefined();
});

test('reading-language changes retranslate and keyboard Send uses automatic translation', async ({ page }) => {
  const sends = await mocks(page);
  await page.goto('/e2e/support/harness/support-translation.html');
  await expect(page.getByText('Hello, I need help.', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Read in: English', exact: true }).click();
  await page.getByRole('option', { name: 'French', exact: true }).click();
  await expect(page.getByText('Bonjour, j’ai besoin d’aide.', { exact: true })).toBeVisible();
  await page.locator('.tiptap').focus();
  await page.keyboard.press('Control+Enter');
  await expect.poll(() => sends.length).toBe(1);
  expect(sends[0].auto_translate).toBe(true);
});
