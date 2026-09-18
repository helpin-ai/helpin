import { expect, test, type Page } from '@playwright/test';

async function mocks(
  page: Page,
  options: {
    available?: boolean;
    incomingFail?: boolean;
    outgoingFail?: boolean;
    settingsFail?: boolean;
    sameLanguage?: boolean;
  } = {},
) {
  const sends: Record<string, unknown>[] = [];
  let failures = options.outgoingFail ? 1 : 0;
  const preference = {
    reading_language: 'en',
    auto_translate_incoming: true,
    auto_translate_outgoing: true,
  };
  const conversation = { customer_language: 'de', translation_mode: 'inherit' };
  let settings = {
    translation_enabled: true,
    translation_incoming_enabled: true,
    translation_outgoing_enabled: true,
    default_agent_language: 'en',
    translation_customer_language: 'de',
  };
  const writes: Record<string, unknown>[] = [];
  let settingsFailures = options.settingsFail ? 1 : 0;
  await page.route('**/api/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const fulfill = (json: unknown, status = 200) =>
      route.fulfill({ json, status });
    if (path.endsWith('/installations')) {
      if (request.method() === 'PATCH') {
        writes.push(request.postDataJSON());
        if (settingsFailures-- > 0)
          return fulfill({ error: 'Could not save settings' }, 503);
        settings = { ...settings, ...request.postDataJSON() };
        preference.reading_language = settings.default_agent_language;
        preference.auto_translate_incoming =
          settings.translation_incoming_enabled;
        preference.auto_translate_outgoing =
          settings.translation_outgoing_enabled;
        conversation.customer_language = settings.translation_customer_language;
      }
      return fulfill({ id: 'installation', workspace_id: 'ws', settings });
    }
    if (path.endsWith('/translation')) {
      if (request.method() !== 'GET')
        throw new Error('Conversation policy must be read only');
      return fulfill({
        available: options.available !== false && settings.translation_enabled,
        unavailable_reason: 'Translation is not configured.',
        languages: { en: 'English', de: 'German', fr: 'French' },
        preference,
        conversation,
      });
    }
    if (path.endsWith('/translation/messages')) {
      if (options.incomingFail)
        return fulfill({ error: 'Translation unavailable' }, 503);
      return fulfill({
        id: 'translated-incoming',
        source_text: options.sameLanguage
          ? 'Hello, I need help.'
          : 'Hallo, ich brauche Hilfe.',
        source_language: options.sameLanguage ? 'en' : 'de',
        target_language: preference.reading_language,
        status: 'ready',
        translated_text:
          preference.reading_language === 'fr'
            ? 'Bonjour, j’ai besoin d’aide.'
            : 'Hello, I need help.',
      });
    }
    if (
      path.endsWith('/conversations/conv/messages') &&
      request.method() === 'POST'
    ) {
      const body = request.postDataJSON();
      sends.push(body);
      if (failures-- > 0)
        return fulfill(
          { error: 'Translation failed. Your reply was not sent.' },
          503,
        );
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
  return { sends, writes };
}

for (const width of [1440, 390]) {
  test(`automatic translation, original toggle and normal Send (${width}px)`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const { sends } = await mocks(page);
    await page.goto('/e2e/support/harness/support-translation.html');
    await expect(
      page.getByText('Hello, I need help.', { exact: true }),
    ).toBeVisible();
    await page
      .getByRole('button', { name: 'Show original', exact: true })
      .click();
    await expect(
      page.getByText('Hallo, ich brauche Hilfe.', { exact: true }),
    ).toBeVisible();
    await page
      .getByRole('button', { name: 'Show translation', exact: true })
      .click();
    await expect(
      page.getByText('Preview translation', { exact: true }),
    ).toHaveCount(0);
    await expect(
      page.getByText('Auto-translate incoming', { exact: true }),
    ).toHaveCount(0);
    await expect(
      page.getByRole('button', {
        name: /Read in:|Send in:|Customer language:|Turn off for this conversation/,
      }),
    ).toHaveCount(0);
    await page.screenshot({
      path: `/tmp/helpin-translation-${width}-light.png`,
      fullPage: true,
    });
    await page.locator('html').evaluate((el) => el.classList.add('dark'));
    await page.screenshot({
      path: `/tmp/helpin-translation-${width}-dark.png`,
      fullPage: true,
    });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
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
test('failed translation retains the draft and stable retry identity', async ({
  page,
}) => {
  const { sends } = await mocks(page, {
    outgoingFail: true,
    incomingFail: true,
  });
  await page.goto('/e2e/support/harness/support-translation.html');
  await expect(
    page.getByText('Translation unavailable. Original shown.'),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Send', exact: true }).click();
  await expect(
    page.getByText('Failed to send message', { exact: true }),
  ).toBeVisible();
  await expect(page.locator('.tiptap')).toHaveText('Hello, we can help.');
  await page.getByRole('button', { name: 'Send', exact: true }).click();
  await expect.poll(() => sends.length).toBe(2);
  expect(sends[1].client_message_id).toBe(sends[0].client_message_id);
});
test('missing provider preserves ordinary messaging', async ({ page }) => {
  const { sends } = await mocks(page, { available: false });
  await page.goto('/e2e/support/harness/support-translation.html');
  await expect(
    page.getByText('Hallo, ich brauche Hilfe.', { exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel('Auto-translate replies')).toHaveCount(0);
  await page.getByRole('button', { name: 'Send', exact: true }).click();
  await expect.poll(() => sends.length).toBe(1);
  expect(sends[0].auto_translate).toBeUndefined();
});

test('workspace reading-language changes retranslate and keyboard Send uses automatic translation', async ({
  page,
}) => {
  const { sends } = await mocks(page);
  await page.goto('/e2e/support/harness/support-translation.html?settings');
  await expect(
    page.getByText('Hello, I need help.', { exact: true }),
  ).toBeVisible();
  await page
    .getByRole('button', { name: 'Reading language: English', exact: true })
    .click();
  await page.getByRole('option', { name: 'French', exact: true }).click();
  await expect(
    page.getByText('Bonjour, j’ai besoin d’aide.', { exact: true }),
  ).toBeVisible();
  await page.locator('.tiptap').focus();
  await page.keyboard.press('Control+Enter');
  await expect.poll(() => sends.length).toBe(1);
  expect(sends[0].auto_translate).toBe(true);
});

for (const width of [1440, 390]) {
  test(`workspace translation settings save globally (${width}px)`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 1100 });
    const { sends, writes } = await mocks(page);
    await page.goto('/e2e/support/harness/support-translation.html?settings');
    await expect(
      page.getByText('Hello, I need help.', { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole('switch', { name: 'Auto-translate', exact: true }),
    ).toBeChecked();
    await page.screenshot({
      path: `/tmp/helpin-global-translation-${width}-light.png`,
      fullPage: true,
    });
    await page.locator('html').evaluate((el) => el.classList.add('dark'));
    await page.screenshot({
      path: `/tmp/helpin-global-translation-${width}-dark.png`,
      fullPage: true,
    });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await page
      .getByRole('switch', { name: 'Auto-translate', exact: true })
      .click();
    await expect(page.getByText('Saved', { exact: true })).toBeVisible();
    expect(writes.at(-1)).toMatchObject({ translation_enabled: false });
    await expect(
      page.getByRole('switch', { name: 'Translate replies', exact: true }),
    ).toBeDisabled();
    await expect(
      page.getByText('Hallo, ich brauche Hilfe.', { exact: true }),
    ).toBeVisible();
    await page.getByRole('button', { name: 'Send', exact: true }).click();
    await expect.poll(() => sends.length).toBe(1);
    expect(sends[0].auto_translate).toBeUndefined();
  });
}

test('workspace policy is read only without support administrator permission', async ({
  page,
}) => {
  const { writes } = await mocks(page);
  await page.goto(
    '/e2e/support/harness/support-translation.html?settings&readonly',
  );
  await expect(
    page.getByRole('switch', { name: 'Auto-translate', exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole('switch', {
      name: 'Translate incoming messages',
      exact: true,
    }),
  ).toBeDisabled();
  await expect(
    page.getByRole('switch', { name: 'Translate replies', exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole('button', {
      name: 'Reading language: English',
      exact: true,
    }),
  ).toBeDisabled();
  expect(writes).toHaveLength(0);
});

test('failed workspace save keeps the change and offers retry', async ({
  page,
}) => {
  const { writes } = await mocks(page, { settingsFail: true });
  await page.goto('/e2e/support/harness/support-translation.html?settings');
  await page
    .getByRole('switch', { name: 'Translate replies', exact: true })
    .click();
  await expect(
    page.getByRole('button', { name: 'Retry', exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole('switch', { name: 'Translate replies', exact: true }),
  ).not.toBeChecked();
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByText('Saved', { exact: true })).toBeVisible();
  expect(writes).toHaveLength(2);
  expect(writes[1]).toMatchObject({ translation_outgoing_enabled: false });
});

test('English incoming messages stay original without a translation label or error', async ({
  page,
}) => {
  await mocks(page, { sameLanguage: true });
  await page.goto('/e2e/support/harness/support-translation.html?english');
  await expect(
    page.getByText('Hello, I need help.', { exact: true }),
  ).toBeVisible();
  await expect(page.getByText('Translating…', { exact: true })).toHaveCount(0);
  await expect(
    page.getByRole('button', { name: 'Show original', exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByText('Translation unavailable. Original shown.'),
  ).toHaveCount(0);
});
