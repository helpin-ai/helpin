import { expect, test, type Page } from "@playwright/test";

async function mockModels(page: Page, admin = true) {
  let profiles = [
    {
      id: "existing",
      workspace_id: "ws",
      scope: "workspace",
      user_id: null,
      name: "Careful answers",
      revision: 3,
      primary: {
        connection_id: "team",
        model: {
          provider: "openai",
          model: "gpt-existing",
          controls: { reasoning_effort: "high" },
        },
      },
      fallback: null,
      hidden_from_ask_agent: false,
    },
  ];
  const writes: unknown[] = [];
  let personalDefault: string | null = null;
  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const send = (json: unknown) => route.fulfill({ json });
    if (path.endsWith("/settings"))
      return send({
        settings: { workspace_id: "ws" },
        teams: [],
        people: [],
        memberships: [],
        user_memberships: [],
      });
    if (path.endsWith("/me"))
      return send({
        permissions: admin
          ? ["workspace.update", "settings.read"]
          : ["settings.read"],
        modules: [],
        membership: { role: admin ? "admin" : "member" },
      });
    if (path.includes("/ai-connections/") && path.includes("/models"))
      return send({
        models: [
          { id: "gpt-existing", name: "Existing model" },
          { id: "gpt-new", name: "New model" },
        ],
        source: "provider",
        stale: false,
      });
    if (path.endsWith("/ai-connections/"))
      return send({
        enabled: true,
        models: [],
        connections: [
          {
            id: "team",
            name: "Team OpenAI",
            provider: "openai",
            scope: "workspace",
            user_id: null,
            status: "connected",
          },
          {id:"personal",name:"My OpenAI",provider:"openai",scope:"personal",user_id:"me",status:"reauthorization_required"},
        ],
      });
    if (path.endsWith("/visibility")) {
      writes.push(request.postDataJSON());
      profiles = profiles.map((p) => ({
        ...p,
        ...request.postDataJSON(),
        revision: p.revision + 1,
      }));
      return send(profiles[0]);
    }
    if (path.endsWith("/ai-profiles/")) return send(profiles);
    if (path.endsWith("/ai-settings/personal")) {
      personalDefault = request.postDataJSON().default_profile_id;
      return send({default_profile_id:"existing",personal_default_profile_id:personalDefault});
    }
    if (path.endsWith("/ai-settings"))
      return send({ default_profile_id: "existing", personal_default_profile_id: personalDefault });
    return send([]);
  });
  return writes;
}

for (const width of [1280, 390])
  test(`connection models preserve variants and visibility at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const writes = await mockModels(page);
    if (width === 390) await page.addInitScript(() => document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.add('dark')));
    await page.goto("/e2e/crm/harness/ai-model-settings.html");
    await expect(
      page.getByRole("button", { name: "Create profile", exact: true }),
    ).toHaveCount(0);
    await expect(page.getByRole("columnheader", {name:"Model",exact:true})).toBeVisible();
    await expect(page.getByRole("tab")).toHaveCount(0);
    await page.screenshot({path:`/tmp/ai-unified-table-${width}.png`,fullPage:true});
    await page.getByRole("button", { name: "Add models",exact:true }).click();
    await page.getByRole("button", { name: "Choose OpenAI",exact:true }).click();
    await expect(page.getByText("My OpenAI", {exact:true})).toBeVisible();
    await expect(page.getByRole("button", {name:"Add another connection",exact:true})).toBeVisible();
    expect(await page.getByRole("dialog").evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
    await page.screenshot({path:`/tmp/ai-add-models-${width}.png`,fullPage:true});
    await page.getByRole("button", { name: "Models for Team OpenAI" }).click();
    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByText("Careful answers", { exact: true }),
    ).toBeVisible();
    await expect(
      dialog.getByRole("switch", { name: "Show New model in Ask Agent" }),
    ).not.toBeChecked();
    const toggle = dialog.getByRole("switch", {
      name: "Show Careful answers in Ask Agent",
    });
    await toggle.click();
    await expect(toggle).not.toBeChecked();
    expect(writes).toEqual([{ revision: 3, hidden_from_ask_agent: true }]);
    await dialog.getByRole("button", { name: "Refresh models" }).click();
    await expect(toggle).not.toBeChecked();
    await dialog
      .getByRole("button", { name: "About model visibility" })
      .focus();
    await expect(page.getByRole("tooltip")).toContainText(
      "Saved agent configurations",
    );
    await page.keyboard.press("Escape");
    await dialog.getByRole("searchbox", { name: "Search models" }).focus();
    expect(
      await dialog.evaluate((el) => el.scrollWidth <= el.clientWidth),
    ).toBe(true);
    if (width === 390)
      await page.locator("html").evaluate((el) => el.classList.add("dark"));
    await page.screenshot({
      path: `/tmp/ai-model-settings-${width}.png`,
      fullPage: true,
    });
  });

test("members have personal setup and read-only shared models", async ({
  page,
}) => {
  await mockModels(page, false);
  await page.goto("/e2e/crm/harness/ai-model-settings.html");
  await expect(page.getByText("Workspace-wide", {exact:true})).toBeVisible();
  await page.getByRole("button", {name:"Actions for Careful answers"}).click();
  await expect(page.getByRole("menuitem",{name:"Set workspace default"})).toHaveCount(0);
  await expect(page.getByRole("menuitem",{name:"Model settings"})).toHaveCount(0);
  await expect(page.getByRole("menuitem",{name:"Hide from Ask Agent"})).toHaveCount(0);
  await page.getByRole("menuitem",{name:"Make my default"}).click();
  await page.getByRole("button", {name:"Actions for Careful answers"}).click();
  await expect(page.getByRole("menuitem",{name:"Use workspace default"})).toBeVisible();
  await page.keyboard.press("Escape");
  await page.getByRole("button",{name:"Manage connections",exact:true}).click();
  await expect(page.getByRole("tab")).toHaveCount(0);
  await page.getByRole("button", { name: "Models for Team OpenAI" }).click();
  await expect(
    page.getByRole("switch", { name: "Show Careful answers in Ask Agent" }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Refresh models" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Edit Careful answers" }),
  ).toHaveCount(0);
});
