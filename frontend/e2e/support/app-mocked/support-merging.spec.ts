import { expect, test, type Page } from "@playwright/test";
import {
  buildConversation,
  CONVERSATION_ID,
  installSupportAppMocks,
  WORKSPACE_SLUG,
} from "../fixtures/supportE2E";

test.setTimeout(90_000);

async function setup(page: Page, nestedSource = false) {
  await installSupportAppMocks(page);
  const source = {
    ...buildConversation(),
    subject: "Unable to sign in",
    merge_revision: 0,
  };
  const target = {
    ...source,
    id: "conv-2",
    display_id: 58,
    subject: "Password reset link expired",
  };
  const messages = (id: string) => [
    {
      id: `${id}-customer`,
      conversation_id: id,
      workspace_id: "ws-1",
      sender_type: "customer",
      sender_display_name: "Visitor Example",
      content:
        id === CONVERSATION_ID
          ? "I cannot sign in after resetting my password."
          : "The reset link expired. Can you send another?",
      is_internal: false,
      message_type: "reply",
      created_at: "2026-10-11T09:00:00Z",
      metadata: "{}",
    },
    {
      id: `${id}-note`,
      conversation_id: id,
      workspace_id: "ws-1",
      sender_type: "user",
      sender_user_id: "user-b",
      sender_display_name: "Bob Agent",
      content: "Checked the account. Waiting for the customer to try again.",
      is_internal: true,
      message_type: "note",
      created_at: "2026-10-11T09:01:00Z",
      metadata: "{}",
    },
  ];
  let merged = false;
  let restored = false;
  let captured: Record<string, unknown> | undefined;
  await page.route(
    (url) => url.pathname === "/api/workspaces/ws-1/me",
    (route) =>
      route.fulfill({
        json: {
          workspace_id: "ws-1",
          modules: ["support"],
          membership: { role: "owner", status: "active" },
          permissions: [
            "workspace.read",
            "support.read",
            "support.edit",
            "support.merge",
            "support.admin",
            "settings.read",
            "ws.connect",
          ],
          team_memberships: [],
        },
      }),
  );
  await page.route(
    (url) => url.pathname.startsWith("/api/support/inbox/conversations/"),
    async (route) => {
      const path = new URL(route.request().url()).pathname;
      const id = path.split("/")[5];
      const suffix = path.split("/")[6];
      const primary = id === target.id ? target : source;
      const edge = {
        id: "edge-1",
        workspace_id: "ws-1",
        source_id: source.id,
        target_id: target.id,
        source,
        target,
        actor_name: "Bob Agent",
        operation_id: "op-1",
        created_at: "2026-10-11T09:02:00Z",
        ...(restored ? { ended_at: "2026-10-11T09:03:00Z" } : {}),
      };
      if (!suffix)
        return route.fulfill({
          json: {
            ...primary,
            ...(merged && !restored && id === source.id
              ? { merged_into_id: target.id, merge_continuation_id: target.id }
              : {}),
          },
        });
      if (suffix === "message-pages")
        return route.fulfill({ json: { data: messages(id), has_more: false } });
      if (suffix === "merge-candidates")
        return route.fulfill({
          json: {
            data: [target],
            has_more: false,
            cross_customer_allowed: true,
          },
        });
      if (suffix === "merge-preview")
        return route.fulfill({
          json: {
            target: primary,
            sources: [
              ...(nestedSource
                ? [{ ...source, id: "conv-child", display_id: 7,
                    subject: "Earlier conversation", merged_into_id: source.id }]
                : []),
              id === target.id ? source : target,
            ],
            preview_token: "review-1",
            participants: ["visitor@example.com"],
            cross_customer: false,
            stops_ai: true,
          },
        });
      if (suffix === "merge") {
        captured = route.request().postDataJSON();
        merged = true;
        return route.fulfill({
          json: {
            conversation_id: id,
            affected_ids: [source.id, target.id],
            merges: [edge],
          },
        });
      }
      if (suffix === "merges")
        return route.fulfill({ json: merged ? [edge] : [] });
      if (suffix === "history")
        return route.fulfill({
          json: { data: messages(target.id), has_more: false },
        });
      if (suffix === "translation" && path.endsWith("/sends"))
        return route.fulfill({ json: [] });
      if (suffix === "translation" && path.endsWith("/messages"))
        return route.fulfill({ json: [] });
      if (suffix === "translation")
        return route.fulfill({
          json: { available: false, preference: {}, conversation: {} },
        });
      return route.fallback();
    },
  );
  await page.route("**/conversation-merges/edge-1/*", async (route) => {
    if (route.request().url().includes("unmerge-preview"))
      return route.fulfill({
        json: {
          sources: [source],
          target,
          preview_token: "undo-review",
          participants: [],
        },
      });
    restored = true;
    return route.fulfill({
      json: {
        conversation_id: source.id,
        affected_ids: [source.id, target.id],
      },
    });
  });
  await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`, {
    waitUntil: "domcontentloaded",
  });
  return { captured: () => captured };
}
async function review(page: Page) {
  await expect(
    page.getByRole("button", { name: "Open conversation actions" }),
  ).toBeVisible({ timeout: 60000 });
  await page.getByRole("button", { name: "Open conversation actions" }).click();
  await page.getByRole("menuitem", { name: "Merge into…" }).click();
  await page
    .getByRole("button", { name: /Password reset link expired/ })
    .click();
  await expect(page.getByRole("dialog")).toContainText("Continues here");
  await expect(
    page.getByRole("button", { name: "Merge into #58" }),
  ).toBeEnabled();
}

test("swapping a merged group keeps its active conversation as the destination", async ({
  page,
}) => {
  await setup(page, true);
  await review(page);
  await expect(page.getByRole("dialog")).toContainText("Earlier conversation");
  await page.getByRole("button", { name: "Swap merge direction" }).click();
  await expect(
    page.getByRole("button", { name: "Merge into #42" }),
  ).toBeEnabled();
});

test("reviews real messages, swaps direction, merges and restores the original", async ({
  page,
}) => {
  const state = await setup(page);
  await review(page);
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText(
    "I cannot sign in after resetting my password.",
  );
  await expect(dialog).toContainText("The reset link expired.");
  await expect(
    dialog.getByText("Checked the account.", { exact: false }).first(),
  ).toBeVisible();
  await expect(dialog.locator('[contenteditable="true"]')).toHaveCount(0);
  await page.getByRole("button", { name: "Swap merge direction" }).click();
  await expect(
    page.getByRole("button", { name: "Merge into #42" }),
  ).toBeEnabled();
  await page.getByRole("button", { name: "Swap merge direction" }).click();
  await page.getByRole("button", { name: "Merge into #58" }).click();
  await expect(page).toHaveURL(/support\/conv-2/);
  expect(state.captured()).toMatchObject({
    source_ids: ["conv-1"],
    preview_token: "review-1",
  });
  await expect(page.getByText("Bob merged a conversation")).toBeVisible();
  await page
    .getByRole("button", { name: "Actions for merged conversation #42" })
    .click();
  await page.getByRole("menuitem", { name: "Unmerge…" }).click();
  await expect(page.getByRole("dialog")).toContainText(
    "Replies sent while merged stay in both histories.",
  );
  await page.getByRole("button", { name: "Unmerge", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page.getByText("Conversation #42 was unmerged", { exact: false }),
  ).toBeVisible();
});

for (const variant of ["dark", "mobile"] as const)
  test(`merge review stays usable in ${variant}`, async ({ page }) => {
    if (variant === "mobile")
      await page.setViewportSize({ width: 390, height: 844 });
    await setup(page);
    if (variant === "dark")
      await page.evaluate(() => document.documentElement.classList.add("dark"));
    await review(page);
    const dialog = page.getByRole("dialog");
    const bounds = await dialog.boundingBox();
    expect(bounds!.width).toBeLessThanOrEqual(page.viewportSize()!.width);
    expect(bounds!.height).toBeLessThanOrEqual(page.viewportSize()!.height);
    if (variant === "mobile") {
      await page.getByRole("tab", { name: "Merge from" }).click();
      await expect(dialog).toContainText(
        "I cannot sign in after resetting my password.",
      );
    }
    await page.screenshot({
      path: `/tmp/helpin-merge-${variant}.png`,
      fullPage: true,
      animations: "disabled",
    });
  });
