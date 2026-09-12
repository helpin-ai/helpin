import { expect, test, type Page } from "@playwright/test";
import {
  buildConversation,
  CONVERSATION_ID,
  installSupportAppMocks,
  WORKSPACE_SLUG,
  SUPPORT_MESSAGES,
} from "../fixtures/supportE2E";

async function setup(page: Page, email = "visitor@example.com", overrides: Record<string, unknown> = {}) {
  await installSupportAppMocks(page);
  await page.route("**/api/support/inbox/installations?**", (route) =>
    route.fulfill({
      json: {
        id: "install-1",
        workspace_id: "ws-1",
        active: true,
        settings: {
          email_fallback_enabled: true,
          email_fallback_delay_secs: 600,
        },
      },
    }),
  );
  await page.route(
    `**/api/support/inbox/conversations/${CONVERSATION_ID}?**`,
    (route) =>
      route.fulfill({
        json: { ...buildConversation(), customer_email: email, ...overrides },
      }),
  );
  const writes: Record<string, unknown>[] = [];
  await page.route(
    `**/api/support/inbox/conversations/${CONVERSATION_ID}/messages?**`,
    (route) => {
      if (route.request().method() !== "POST") return route.fallback();
      const data = route.request().postDataJSON();
      writes.push(data);
      return route.fulfill({
        json: {
          id: `reply-${writes.length}`,
          workspace_id: "ws-1",
          conversation_id: CONVERSATION_ID,
          sender_type: "user",
          sender_user_id: "user-b",
          message_type: "reply",
          ...data,
          created_at: new Date().toISOString(),
        },
      });
    },
  );
  await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`);
  const composer = page.locator("[data-support-reply-composer]:visible");
  await expect(composer.locator('[contenteditable="true"]')).toBeVisible({
    timeout: 45000,
  });
  return { composer, writes };
}

for (const width of [1440, 390]) {
  test(`reply send selector chooses real channels (${width}px)`, async ({
    page,
  }) => {
    test.setTimeout(90000);
    await page.setViewportSize({ width, height: 900 });
    const { composer, writes } = await setup(page);
    const selector = composer.getByRole("button", {
      name: /^Sending options:/,
    });
    const sendBounds = await composer
      .getByRole("button", { name: "Send", exact: true })
      .boundingBox();
    const composerBounds = await composer.boundingBox();
    expect(sendBounds!.x + sendBounds!.width).toBeLessThanOrEqual(
      composerBounds!.x + composerBounds!.width,
    );
    await selector.click();
    await expect(page.getByRole("option")).toHaveCount(3);
    await page.screenshot({ path: `/tmp/helpin-send-options-${width}.png` });
    await page.getByRole("option", { name: /^Email only/ }).click();
    await expect(selector).toHaveAttribute(
      "aria-label",
      "Sending options: Email only",
    );
    await composer
      .locator('[contenteditable="true"]')
      .fill("An email-only reply");
    await page.screenshot({ path: `/tmp/helpin-reply-composer-${width}.png` });
    await composer.getByRole("button", { name: "Send", exact: true }).click();
    await expect.poll(() => writes.length).toBe(1);
    expect(writes[0]).toMatchObject({
  expect(writes[0]).not.toHaveProperty('email_subject');
      delivery_mode: "email_only",
      channels: ["email"],
    });
    await expect(page.getByRole("alertdialog")).toHaveCount(0);
    await selector.click();
    await page.getByRole("option", { name: /^Chat only/ }).click();
    await composer
      .locator('[contenteditable="true"]')
      .fill("A chat-only reply");
    await composer.getByRole("button", { name: "Send", exact: true }).click();
    await expect.poll(() => writes.length).toBe(2);
    expect(writes[1]).toMatchObject({
      delivery_mode: "chat_only",
      channels: ["chat"],
    });
    expect(writes[1]).not.toHaveProperty("cc_emails");
    await selector.click();
    await page.getByRole("option", { name: /^Chat \+ email/ }).click();
    await composer.locator('[contenteditable="true"]').fill("Both channels");
    await composer.getByRole("button", { name: "Send", exact: true }).click();
    await expect.poll(() => writes.length).toBe(3);
    expect(writes[2]).toMatchObject({
      delivery_mode: "chat_and_email",
      channels: ["chat", "email"],
    });
  });
}

async function presence(page: Page, visitors: string[]) {
  await page.evaluate((visitors) => window.__supportE2E?.emit({ type: 'support:online_visitors', data: { visitors } }), visitors);
}

test("presence adds email once and sends the displayed promise without confirmation", async ({ page }) => {
  test.setTimeout(90000);
  const { composer, writes } = await setup(page);
  const selector = composer.getByRole('button', { name: /^Sending options:/ });
  await expect(selector).toHaveAttribute('aria-label', 'Sending options: Chat only');
  await presence(page, ['anon-1']);
  await expect(composer.getByText(/Reply will .*emailed/)).toHaveCount(0);
  await composer.locator('[contenteditable="true"]').fill('A reply in progress');
  await presence(page, []);
  await expect(selector).toHaveAttribute('aria-label', 'Sending options: Chat + email');
  await expect(composer).toContainText('Reply will also be emailed to visitor@example.com');
  await expect(composer.getByRole('textbox', { name: 'Subject', exact: true })).toHaveCount(0);
  await presence(page, ['anon-1']);
  await expect(selector).toHaveAttribute('aria-label', 'Sending options: Chat + email');
  await expect(composer).toContainText('Reply will also be emailed to visitor@example.com');
  await composer.getByRole('button', { name: 'Send', exact: true }).click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0]).toMatchObject({ delivery_mode: 'chat_and_email', channels: ['chat', 'email'] });
  expect(writes[0]).not.toHaveProperty('email_subject');
  await expect(page.getByRole('alertdialog')).toHaveCount(0);
  await expect(selector).toHaveAttribute('aria-label', 'Sending options: Chat only');
});

test("manual delivery choice survives sends, notes, and reload without a subject editor", async ({ page }) => {
  test.setTimeout(90000);
  const { composer, writes } = await setup(page);
  const selector = composer.getByRole('button', { name: /^Sending options:/ });
  await selector.click();
  await page.getByRole('option', { name: /^Email only/ }).click();
  await expect(composer).toContainText('Reply will only be emailed to visitor@example.com');
  const subject = composer.getByRole('textbox', { name: 'Subject', exact: true });
  await composer.getByRole('button', { name: 'Note', exact: true }).click();
  await expect(subject).toHaveCount(0);
  await composer.getByRole('button', { name: 'Reply', exact: true }).click();
  await expect(subject).toHaveCount(0);
  await page.reload();
  await expect(selector).toBeVisible({ timeout: 45000 });
  await expect(subject).toHaveCount(0);
  await presence(page, []);
  await presence(page, ['anon-1']);
  await expect(selector).toHaveAttribute('aria-label', 'Sending options: Email only');
  await composer.locator('[contenteditable="true"]').fill('Updated details');
  await composer.getByRole('button', { name: 'Send', exact: true }).click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0]).toMatchObject({ delivery_mode: 'email_only' });
  expect(writes[0]).not.toHaveProperty('email_subject');
  await expect(selector).toHaveAttribute('aria-label', 'Sending options: Email only');
  await expect(subject).toHaveCount(0);
  await selector.click();
  await page.getByRole('option', { name: /^Chat only/ }).click();
  await presence(page, []);
  await expect(selector).toHaveAttribute('aria-label', 'Sending options: Chat only');
  await expect(subject).toHaveCount(0);
});

test("email options explain missing address and notes hide the channel control", async ({
  page,
}) => {
  test.setTimeout(90000);
  const { composer } = await setup(page, "");
  await composer.getByRole("button", { name: /^Sending options:/ }).click();
  await expect(
    page.getByRole("option", { name: /^Email only/ }),
  ).toHaveAttribute("aria-disabled", "true");
  await expect(
    page.getByRole("option", { name: /^Chat \+ email/ }),
  ).not.toContainText("No email address");
  const unavailableEmail = page.getByRole("option", { name: /^Email only/ });
  await unavailableEmail.hover();
  await expect(page.getByRole("tooltip", { name: "No email address", exact: true })).toBeVisible();
  const unavailableBounds = await unavailableEmail.boundingBox();
  await page.mouse.click(unavailableBounds!.x + unavailableBounds!.width / 2, unavailableBounds!.y + unavailableBounds!.height / 2);
  await expect(page.getByRole("option")).toHaveCount(3);
  await expect(composer.getByRole("button", { name: "Sending options: Chat only", exact: true })).toBeVisible();
  await page.mouse.move(0, 0);
  // Start keyboard coverage from a fresh menu, without the pointer-dismissed tooltip state.
  await page.keyboard.press("Escape");
  await composer.getByRole("button", { name: /^Sending options:/ }).press("Enter");
  await unavailableEmail.locator('[tabindex="0"]').focus();
  await expect(page.getByRole("tooltip", { name: "No email address", exact: true })).toBeVisible();
  await unavailableEmail.locator('[tabindex="0"]').press("Enter");
  await expect(page.getByRole("option")).toHaveCount(3);
  await page.keyboard.press("Escape");
  await composer.getByRole("button", { name: "Note", exact: true }).click();
  await expect(
    composer.getByRole("button", { name: /^Sending options:/ }),
  ).toHaveCount(0);
  await expect(
    composer.getByRole("button", { name: "Add Note", exact: true }),
  ).toBeVisible();
});

test("email-only selection survives a draft reload and keyboard send uses it", async ({
  page,
}) => {
  test.setTimeout(90000);
  const { composer, writes } = await setup(page);
  await composer.getByRole("button", { name: /^Sending options:/ }).click();
  await page.getByRole("option", { name: /^Email only/ }).click();
  await page.setViewportSize({ width: 390, height: 900 });
  await expect(
    composer.getByRole("button", {
      name: "Sending options: Email only",
      exact: true,
    }),
  ).toBeVisible();
  await page.evaluate(() => window.__supportE2E?.clearSentMessages());
  await composer
    .locator('[contenteditable="true"]')
    .pressSequentially("Keep this email private from chat");
  const typed = await page.evaluate(() =>
    window.__supportE2E
      ?.getSentMessages()
      .filter(
        (event) =>
          event.type === "support:typing:start" ||
          event.type === "support:typing:update",
      ),
  );
  expect(typed).toEqual([]);
  await expect
    .poll(() => page.evaluate(() => Object.values(localStorage).join(" ")))
    .toContain("Keep this email private from chat");
  await page.reload();
  await expect(
    composer.getByRole("button", {
      name: "Sending options: Email only",
      exact: true,
    }),
  ).toBeVisible({ timeout: 45000 });
  const editor = composer.locator('[contenteditable="true"]');
  await expect(editor).toContainText("Keep this email private from chat");
  await editor.press("Control+Enter");
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0]).toMatchObject({
  expect(writes[0]).not.toHaveProperty('email_subject');
    delivery_mode: "email_only",
    channels: ["email"],
  });
});

test("failed send retains the email-only draft and delivery choice", async ({
  page,
}) => {
  test.setTimeout(90000);
  const { composer, writes } = await setup(page);
  await page.route(
    `**/api/support/inbox/conversations/${CONVERSATION_ID}/messages?**`,
    (route) => {
      if (route.request().method() !== "POST") return route.fallback();
      return route.fulfill({
        status: 503,
        json: { error: "Email temporarily unavailable" },
      });
    },
  );
  await composer.getByRole("button", { name: /^Sending options:/ }).click();
  await page.getByRole("option", { name: /^Email only/ }).click();
  await composer
    .locator('[contenteditable="true"]')
    .fill("Retry this same email");
  await composer.getByRole("button", { name: "Send", exact: true }).click();
  await expect(
    page.getByText("Email temporarily unavailable", { exact: true }).first(),
  ).toBeVisible();
  await expect(composer.locator('[contenteditable="true"]')).toContainText(
    "Retry this same email",
  );
  await expect(
    composer.getByRole("button", {
      name: "Sending options: Email only",
      exact: true,
    }),
  ).toBeVisible();
  await expect(composer.getByRole("textbox", { name: "Subject", exact: true })).toHaveCount(0);
  expect(writes).toHaveLength(0);
});


test("failed in-flight send retains its original chat-only intent after the visitor goes offline", async ({ page }) => {
  test.setTimeout(90000);
  const { composer } = await setup(page);
  await presence(page, ['anon-1']);
  let finish!: () => void;
  const pending = new Promise<void>((resolve) => { finish = resolve; });
  let started = false;
  await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}/messages?**`, async (route) => {
    if (route.request().method() !== 'POST') return route.fallback();
    started = true;
    await pending;
    await route.fulfill({ status: 503, json: { error: 'Retry delivery' } });
  });
  await composer.locator('[contenteditable="true"]').fill('Keep this chat reply');
  await composer.getByRole('button', { name: 'Send', exact: true }).click();
  await expect.poll(() => started).toBe(true);
  await presence(page, []);
  finish();
  await expect(composer.getByRole('button', { name: 'Send', exact: true })).toBeEnabled();
  await expect(composer.getByRole('button', { name: /^Sending options:/ })).toHaveAttribute('aria-label', 'Sending options: Chat only');
  await expect(composer.locator('[contenteditable="true"]')).toContainText('Keep this chat reply');
});


test("email-origin replies show Cc and let the server choose the subject", async ({ page }) => {
  test.setTimeout(90000);
  const { composer, writes } = await setup(page, 'visitor@example.com', { source: 'email', anonymous_id: '', email_cc: ['copy@example.com'] });
  await expect(composer).toContainText('Reply will only be emailed to visitor@example.com. Cc: copy@example.com.');
  const selector = composer.getByRole('button', { name: /^Sending options:/ });
  await selector.click();
  await expect(page.getByRole('option', { name: /^Chat only/ })).toHaveAttribute('aria-disabled', 'true');
  await page.keyboard.press('Escape');
  const subject = composer.getByRole('textbox', { name: 'Subject', exact: true });
  await composer.locator('[contenteditable="true"]').fill('Details for all recipients');
  await expect(composer.getByRole('button', { name: 'Send', exact: true })).toBeEnabled();
  await composer.getByRole('button', { name: 'Send', exact: true }).click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0]).toMatchObject({ delivery_mode: 'email_only', cc_emails: ['copy@example.com'] });
  expect(writes[0]).not.toHaveProperty('email_subject');
  await expect(subject).toHaveCount(0);
});

test("Undo restores the attachment and delivery mode without a subject editor", async ({ page }) => {
  test.setTimeout(90000);
  const { composer } = await setup(page);
  const original = { id: 'undo-message', workspace_id: 'ws-1', conversation_id: CONVERSATION_ID, sender_type: 'user', sender_user_id: 'user-b',
    message_type: 'reply', content: 'Original email reply', created_at: new Date().toISOString(),
    cancellable_until: new Date(Date.now() + 600000).toISOString(),
    metadata: JSON.stringify({ delivery_mode: 'email_only', email_subject: 'Original subject', email_delivery_status: 'queued' }),
    attachments: [{ id: 'file-undo', file_name: 'receipt.pdf', file_type: 'application/pdf', file_size: 120, file_key: 'support/receipt.pdf' }] };
  let removed = false;
  await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}/message-pages?**`, (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({ json: { data: removed ? SUPPORT_MESSAGES : [...SUPPORT_MESSAGES, original], has_more: false } });
  });
  await page.route('**/api/support/inbox/**undo-message**', (route) => {
    if (route.request().method() !== 'DELETE') return route.fallback();
    removed = true;
    return route.fulfill({ json: { markdown: original.content } });
  });
  await page.reload();
  const selector = composer.getByRole('button', { name: /^Sending options:/ });
  await expect(selector).toBeVisible({ timeout: 45000 });
  await selector.click();
  await page.getByRole('option', { name: /^Chat only/ }).click();
  await page.evaluate(() => window.__supportE2E?.clearSentMessages());
  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await expect(composer.locator('[contenteditable="true"]')).toContainText(original.content);
  await expect(selector).toHaveAttribute('aria-label', 'Sending options: Email only');
  await expect(composer.getByRole('textbox', { name: 'Subject', exact: true })).toHaveCount(0);
  await expect(composer.getByRole('button', { name: 'Remove receipt.pdf', exact: true })).toBeVisible();
  const typing = await page.evaluate(() => window.__supportE2E?.getSentMessages().filter((event) => event.type === 'support:typing:start' || event.type === 'support:typing:update'));
  expect(typing).toEqual([]);
});
