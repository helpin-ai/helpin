import { expect, test, type Page } from "@playwright/test";
import {
  buildConversation,
  CONVERSATION_ID,
  installSupportAppMocks,
  WORKSPACE_SLUG,
} from "../fixtures/supportE2E";

async function setup(page: Page, email = "visitor@example.com") {
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
        json: { ...buildConversation(), customer_email: email },
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
    await composer.getByRole("button", { name: "Send", exact: true }).click();
    await expect.poll(() => writes.length).toBe(1);
    expect(writes[0]).toMatchObject({
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

test("untouched send options preserve the offline banner and confirmation", async ({
  page,
}) => {
  test.setTimeout(90000);
  const { composer, writes } = await setup(page);
  await composer
    .locator('[contenteditable="true"]')
    .fill("Existing offline flow");
  await expect(composer).toContainText(
    "User is offline. Replies sent here will also be queued as an email to",
  );
  await composer.getByRole("button", { name: "Send", exact: true }).click();
  const confirm = page.getByRole("alertdialog");
  await expect(confirm).toContainText("Send this reply by email too?");
  await expect(confirm).toContainText(
    "skip the email if the visitor comes back online before it sends",
  );
  expect(writes).toHaveLength(0);
  await confirm
    .getByRole("button", { name: "Send Reply", exact: true })
    .click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0]).not.toHaveProperty("delivery_mode");
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
  ).toContainText("No email address");
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
  expect(writes).toHaveLength(0);
});
