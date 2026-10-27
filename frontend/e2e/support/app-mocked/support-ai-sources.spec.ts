import { expect, test } from "@playwright/test";
import {
  CONVERSATION_ID,
  installSupportAppMocks,
  SUPPORT_MESSAGES,
  WORKSPACE_ID,
  WORKSPACE_SLUG,
} from "../fixtures/supportE2E";

const AI_REPLY = {
  id: "msg-ai-sources",
  workspace_id: WORKSPACE_ID,
  conversation_id: CONVERSATION_ID,
  sender_type: "ai",
  message_type: "reply",
  content: "You can export invoices from Billing.",
  is_internal: false,
  via_channel: "widget",
  metadata: JSON.stringify({
    ai_auto_reply: true,
    ai_sources: [
      { docId: "doc-1", title: "Exporting invoices", url: "https://help.example.com/invoices" },
      { docId: "doc-2", title: "Billing settings" },
    ],
  }),
  created_at: "2026-03-27T19:58:00Z",
  updated_at: "2026-03-27T19:58:00Z",
};

test("AI sources list closes on an outside click and Escape", async ({ page }) => {
  test.setTimeout(90000);
  await installSupportAppMocks(page);
  const messages = [...SUPPORT_MESSAGES, AI_REPLY];
  await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}/messages?**`, (route) =>
    route.request().method() === "GET" ? route.fulfill({ json: messages }) : route.fallback(),
  );
  await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}/message-pages?**`, (route) =>
    route.fulfill({ json: { data: messages, has_more: false } }),
  );
  await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`);

  const trigger = page.getByRole("button", { name: "2 sources" });
  await expect(trigger).toBeVisible({ timeout: 45000 });
  const source = page.getByRole("link", { name: "Exporting invoices" });

  await trigger.click();
  await expect(source).toBeVisible();
  await page.getByText("You can export invoices from Billing.").click();
  await expect(source).toHaveCount(0);
  await expect(trigger).toHaveAttribute("aria-expanded", "false");

  await trigger.click();
  await expect(page.getByText("Billing settings")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByText("Billing settings")).toHaveCount(0);
});
