import { expect, test, type Page } from "@playwright/test";

async function installMocks(
  page: Page,
  options: {
    failSave?: boolean;
    readOnly?: boolean;
    conflictOnce?: boolean;
    failRefreshAfterSave?: boolean;
    multiplePipelines?: boolean;
    motion?: string;
  } = {},
) {
  const stages = [
    "Lead found",
    "Contacted",
    "Demo scheduled",
    "Qualified to buy",
    "Presentation scheduled",
    "Decision maker bought-in",
    "Contract sent",
    "Closed won",
    "Closed lost",
  ].map((name, position) => ({
    id: `stage-${position}`,
    pipeline_id: "sales",
    name,
    position,
    stage_type: position === 7 ? "won" : position === 8 ? "lost" : "open",
    probability: [0, 10, 50, 40, 60, 80, 90, 100, 0][position],
    deal_count: position === 0 ? 12 : 0,
    created_at: "2026-09-01T10:00:00Z",
    updated_at: "2026-09-01T10:00:00Z",
  }));
  const pipeline = {
    id: "sales",
    workspace_id: "ws-pipelines",
    name: "Outbound sales pipeline",
    is_default: true,
    default_commercial_motion: options.motion ?? "new_business",
    deal_count: 12,
    stages,
    updated_at: "2026-09-01T10:00:00Z",
  };
  const writes: Record<string, any>[] = [];
  const list = [pipeline];
  if (options.multiplePipelines)
    list.push({
      ...pipeline,
      id: "renewals",
      name: "Renewals pipeline",
      is_default: false,
    });
  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path.endsWith("/me"))
      return route.fulfill({
        json: {
          permissions: options.readOnly
            ? ["crm.read"]
            : ["crm.read", "crm.admin"],
          role: options.readOnly ? "viewer" : "admin",
        },
      });
    if (path.endsWith("/pipelines") && request.method() === "POST") {
      const body = request.postDataJSON();
      writes.push(body);
      const created = { ...pipeline, ...body, id: "copied" };
      list.push(created);
      return route.fulfill({ json: created });
    }
    if (path.endsWith("/pipelines") && request.method() === "GET") {
      if (options.failRefreshAfterSave && writes.length)
        return route.fulfill({
          status: 503,
          json: { error: "Temporarily unavailable" },
        });
      return route.fulfill({ json: list });
    }
    if (path.endsWith("/pipelines/sales") && request.method() === "PUT") {
      const body = request.postDataJSON();
      writes.push(body);
      if (options.conflictOnce && writes.length === 1) {
        pipeline.updated_at = "2026-09-02T10:00:00Z";
        pipeline.stages[1].name = "Reached out";
        return route.fulfill({
          status: 409,
          json: { error: "Pipeline changed. Refresh and try again." },
        });
      }
      if (options.failSave)
        return route.fulfill({
          status: 409,
          json: { error: "Pipeline changed. Refresh and try again." },
        });
      if (body.stages) {
        pipeline.stages = body.stages.map((stage: any, i: number) => ({
          ...stages.find((s) => s.id === stage.id),
          ...stage,
          id: stage.id ?? `new-${i}`,
          deal_count: stages.find((s) => s.id === stage.id)?.deal_count ?? 0,
        }));
      }
      if (body.name) pipeline.name = body.name;
      if (body.default_commercial_motion)
        pipeline.default_commercial_motion = body.default_commercial_motion;
      return route.fulfill({ json: pipeline });
    }
    return route.fulfill({ json: {} });
  });
  return { writes, pipeline };
}

const url = "/e2e/crm/harness/pipelines.html";

test("pipelines start collapsed with stage controls on the right and contextual actions", async ({ page }) => {
  await installMocks(page, { multiplePipelines: true });
  await page.goto(url);
  const sales = page.locator("#pipeline-toggle-sales");
  const renewals = page.locator("#pipeline-toggle-renewals");
  const section = page.locator("section").filter({ has: sales });
  const actions = section.getByRole("button", { name: "Edit pipeline", exact: true }).locator("..");
  await expect(sales).toHaveAttribute("aria-expanded", "false");
  await expect(renewals).toHaveAttribute("aria-expanded", "false");
  await expect(sales).toHaveText("Show stages");
  await expect(page.getByRole("region", { name: "Open stages", exact: true })).toHaveCount(0);
  await expect(actions).toHaveCSS("opacity", "0");
  await page.screenshot({ path: "/tmp/helpin-pipelines-collapsed.png", fullPage: true });
  const headingBounds = await page.locator("#pipeline-sales").boundingBox();
  const toggleBounds = await sales.boundingBox();
  expect(toggleBounds!.x).toBeGreaterThan(headingBounds!.x + headingBounds!.width);
  await section.hover();
  await expect(actions).toHaveCSS("opacity", "1");
  await page.mouse.move(0, 0);
  await expect(actions).toHaveCSS("opacity", "0");
  await sales.focus();
  await expect(actions).toHaveCSS("opacity", "1");
  await sales.press("Enter");
  await expect(sales).toHaveAttribute("aria-expanded", "true");
  await expect(sales).toHaveText("Hide stages");
  await page.getByRole("button", { name: "Create pipeline", exact: true }).focus();
  await expect(actions).toHaveCSS("opacity", "1");
  await renewals.click();
  await expect(sales).toHaveAttribute("aria-expanded", "false");
  await expect(renewals).toHaveAttribute("aria-expanded", "true");
  await expect(page.getByRole("region", { name: "Open stages", exact: true })).toHaveCount(1);
  await renewals.press("Enter");
  await expect(renewals).toHaveAttribute("aria-expanded", "false");
  await expect(page.getByRole("region", { name: "Open stages", exact: true })).toHaveCount(0);
  await expect(page.locator("#pipeline-selector")).toHaveCount(0);
  await expect(page.getByText("Changes save automatically", { exact: true })).toHaveCount(0);
});

test("inline stage fields save on blur or Enter and cancel with Escape", async ({
  page,
}) => {
  const { writes } = await installMocks(page);
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  const name = page.getByRole("textbox", {
    name: "Stage name: Contacted",
    exact: true,
  });
  await name.fill("Contact made");
  await name.press("Enter");
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].stages[1]).toMatchObject({
    id: "stage-1",
    name: "Contact made",
    probability: 10,
  });
  const probability = page.getByRole("spinbutton", {
    name: "Win probability: Contact made",
    exact: true,
  });
  await probability.fill("101");
  await probability.press("Tab");
  await expect(probability).toHaveAttribute("aria-invalid", "true");
  expect(writes).toHaveLength(1);
  await probability.fill("35");
  await probability.press("Tab");
  await expect.poll(() => writes.length).toBe(2);
  expect(writes[1].stages[1].probability).toBe(35);
  const renamed = page.getByRole("textbox", {
    name: "Stage name: Contact made",
    exact: true,
  });
  await renamed.fill("Discard this");
  await renamed.press("Escape");
  await expect(renamed).toHaveValue("Contact made");
  expect(writes).toHaveLength(2);
  await expect(
    page.getByRole("button", { name: "Edit Contact made", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("spinbutton", {
      name: "Win probability: Closed won",
      exact: true,
    }),
  ).toHaveCount(0);
});

test("failed inline saves retain the draft and allow retry after a conflict", async ({
  page,
}) => {
  const { writes } = await installMocks(page, { conflictOnce: true });
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  const name = page.getByRole("textbox", {
    name: "Stage name: Contacted",
    exact: true,
  });
  await name.fill("Contact made");
  await name.press("Enter");
  await expect(
    page.getByText("Pipeline changed. Refresh and try again."),
  ).toBeVisible();
  const refreshed = page.getByRole("textbox", {
    name: "Stage name: Reached out",
    exact: true,
  });
  await expect(refreshed).toHaveValue("Contact made");
  await refreshed.focus();
  await refreshed.press("Enter");
  await expect.poll(() => writes.length).toBe(2);
  expect(writes[1].expected_updated_at).toBe("2026-09-02T10:00:00Z");
  await expect(
    page.getByRole("textbox", {
      name: "Stage name: Contact made",
      exact: true,
    }),
  ).toHaveValue("Contact made");
});

test("saving through Edit replaces an invalid inline draft", async ({
  page,
}) => {
  await installMocks(page);
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  const probability = page.getByRole("spinbutton", {
    name: "Win probability: Contacted",
    exact: true,
  });
  await probability.fill("101");
  await probability.press("Tab");
  await expect(probability).toHaveAttribute("aria-invalid", "true");
  await page
    .getByRole("button", { name: "Edit Contacted", exact: true })
    .click();
  await page.getByLabel("Win probability (%)", { exact: true }).fill("35");
  await page.getByRole("button", { name: "Save stage", exact: true }).click();
  await expect(page.locator('[data-slot="dialog-content"]')).not.toBeVisible();
  await expect(probability).toHaveValue("35");
  await expect(probability).toHaveAttribute("aria-invalid", "false");
});

test("touch users can access stage guidance and removal", async ({
  browser,
}) => {
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    hasTouch: true,
    isMobile: true,
  });
  const page = await context.newPage();
  await installMocks(page);
  await page.goto(`http://127.0.0.1:5193${url}?dark`);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  await expect(
    page
      .getByRole("button", { name: "Delete Contacted", exact: true })
      .locator(".."),
  ).toHaveCSS("opacity", "1");
  await page
    .getByRole("button", {
      name: "About win probability for contacted",
      exact: true,
    })
    .tap();
  await expect(
    page.getByRole("tooltip", { name: /Win probability estimates/ }),
  ).toBeVisible();
  await page
    .getByRole("heading", { name: "Deal pipelines", exact: true })
    .tap();
  await page
    .getByRole("button", { name: "About won outcomes", exact: true })
    .tap();
  await expect(
    page.getByRole("tooltip", { name: /Drag a handle/ }),
  ).toContainText("Won and lost outcomes stay at the end.");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await context.close();
});

test("stage removal appears on hover or keyboard focus and guidance lives in tooltips", async ({
  page,
}) => {
  await installMocks(page);
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  const remove = page.getByRole("button", {
    name: "Delete Contacted",
    exact: true,
  });
  const wrapper = remove.locator("..");
  await expect(wrapper).toHaveCSS("opacity", "0");
  await page
    .getByRole("textbox", { name: "Stage name: Contacted", exact: true })
    .hover();
  await expect(wrapper).toHaveCSS("opacity", "1");
  await page.mouse.move(0, 0);
  await remove.focus();
  await expect(wrapper).toHaveCSS("opacity", "1");
  await page
    .getByRole("button", { name: "About win probability", exact: true })
    .hover();
  await expect(
    page.getByRole("tooltip", { name: /Win probability estimates/ }),
  ).toContainText("A deal’s own probability takes priority.");
});

test("moves a stage several positions in one action and preserves stage identities", async ({
  page,
}) => {
  const { writes } = await installMocks(page);
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  await page.getByRole("combobox", { name: "Position of Lead found" }).click();
  await page
    .getByRole("option", { name: "7 · Contract sent", exact: true })
    .click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].stages.map((s: any) => s.id)).toEqual([
    "stage-1",
    "stage-2",
    "stage-3",
    "stage-4",
    "stage-5",
    "stage-6",
    "stage-0",
    "stage-7",
    "stage-8",
  ]);
  expect(writes[0].expected_updated_at).toBe("2026-09-01T10:00:00Z");
  await expect(
    page.getByRole("combobox", { name: "Position of Lead found" }),
  ).toHaveText("7");
});

test("deleting a populated stage requires an explicit same-outcome destination", async ({
  page,
}) => {
  const { writes } = await installMocks(page);
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Delete Lead found", exact: true })
    .click();
  const dialog = page.locator('[data-slot="dialog-content"]');
  await expect(dialog.getByText(/12 deals/)).toBeVisible();
  await expect(
    dialog.getByRole("button", { name: "Move deals & delete stage" }),
  ).toBeDisabled();
  await dialog.getByRole("combobox", { name: "Move deals to" }).click();
  await expect(page.getByRole("option", { name: "Closed won" })).toHaveCount(0);
  await page.getByRole("option", { name: "Contacted", exact: true }).click();
  await dialog
    .getByRole("button", { name: "Move deals & delete stage" })
    .click();
  await expect(dialog).not.toBeVisible();
  expect(writes[0].stage_migrations).toEqual({ "stage-0": "stage-1" });
  expect(writes[0].stages).toHaveLength(8);
});

test("failed saves keep the form and never show a success notification", async ({
  page,
}) => {
  await installMocks(page, { failSave: true });
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Edit Contacted", exact: true })
    .click();
  await page.getByLabel("Stage name", { exact: true }).fill("Contact made");
  await page.getByRole("button", { name: "Save stage", exact: true }).click();
  await expect(page.locator('[data-slot="dialog-content"]')).toBeVisible();
  await expect(page.getByLabel("Stage name", { exact: true })).toHaveValue(
    "Contact made",
  );
  await expect(
    page.getByText("Pipeline changed. Refresh and try again."),
  ).toBeVisible();
  await expect(page.getByText("Stage updated", { exact: true })).toHaveCount(0);
});

test("stage editor is usable in a narrow dark view and respects read-only access", async ({
  page,
}) => {
  await installMocks(page, { readOnly: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${url}?dark`);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  await expect(page.getByText("Lead found", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Delete Lead found", exact: true }),
  ).toHaveCount(0);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "/tmp/helpin-pipelines-mobile.png",
    fullPage: true,
  });
});

test("keyboard dragging moves across several stages and preserves won/lost outcomes", async ({
  page,
}) => {
  const { writes } = await installMocks(page);
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  const handle = page.getByRole("button", {
    name: "Drag Lead found",
    exact: true,
  });
  await handle.focus();
  await page.keyboard.press("Space");
  await expect(handle).toHaveAttribute("aria-pressed", "true");
  for (const position of [2, 3, 4]) {
    await page.keyboard.press("ArrowDown");
    await expect(
      page.getByText(`Lead found is over position ${position} of 7.`, {
        exact: true,
      }),
    ).toBeAttached();
  }
  await page.keyboard.press("Space");
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].stages.map((s: any) => s.id)).toEqual([
    "stage-1",
    "stage-2",
    "stage-3",
    "stage-0",
    "stage-4",
    "stage-5",
    "stage-6",
    "stage-7",
    "stage-8",
  ]);
  await expect(
    page.getByRole("button", { name: "Delete Closed won", exact: true }),
  ).toBeDisabled();
  await page.screenshot({
    path: "/tmp/helpin-pipelines-desktop.png",
    fullPage: true,
  });
});

test("creates a pipeline by copying stages without copying stage IDs or deals", async ({
  page,
}) => {
  const { writes } = await installMocks(page);
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Create pipeline", exact: true })
    .click();
  const dialog = page.locator('[data-slot="dialog-content"]');
  await page.getByLabel("Pipeline name", { exact: true }).fill("Partner sales");
  await page.getByRole("combobox", { name: "Start with" }).click();
  await page
    .getByRole("option", { name: "Copy stages from Outbound sales pipeline" })
    .click();
  await dialog
    .getByRole("button", { name: "Create pipeline", exact: true })
    .click();
  await expect(dialog).not.toBeVisible();
  expect(writes[0].name).toBe("Partner sales");
  expect(writes[0].default_commercial_motion).toBe("new_business");
  expect(writes[0].stages).toHaveLength(9);
  expect(writes[0].stages[0]).toEqual({
    name: "Lead found",
    stage_type: "open",
    probability: 0,
    position: 0,
  });
});

test("adds a stage at a chosen position and validates probability", async ({
  page,
}) => {
  const { writes } = await installMocks(page);
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  await page.getByRole("button", { name: "Add stage", exact: true }).click();
  const dialog = page.locator('[data-slot="dialog-content"]');
  await page.getByLabel("Stage name", { exact: true }).fill("Discovery call");
  await page.getByLabel("Win probability (%)", { exact: true }).fill("101");
  await expect(
    dialog.getByRole("button", { name: "Add stage", exact: true }),
  ).toBeDisabled();
  await page.getByLabel("Win probability (%)", { exact: true }).fill("25");
  await page.getByRole("combobox", { name: "Position in open stages" }).click();
  await page.getByRole("option", { name: "3 · After Contacted" }).click();
  await dialog.getByRole("button", { name: "Add stage", exact: true }).click();
  await expect(dialog).not.toBeVisible();
  expect(writes[0].stages[2]).toEqual({
    name: "Discovery call",
    stage_type: "open",
    probability: 25,
    position: 2,
  });
});

test("empty stage deletion and mobile editing keep their controls visible", async ({
  page,
}) => {
  const { writes } = await installMocks(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  await expect(
    page.getByRole("combobox", { name: "Position of Lead found" }),
  ).toBeVisible();
  await page.screenshot({
    path: "/tmp/helpin-pipelines-mobile-editable.png",
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Delete Contacted", exact: true })
    .click();
  const dialog = page.locator('[data-slot="dialog-content"]');
  await expect(
    dialog.getByText("This stage has no deals. Removing it cannot be undone."),
  ).toBeVisible();
  await dialog
    .getByRole("button", { name: "Delete stage", exact: true })
    .click();
  await expect(dialog).not.toBeVisible();
  expect(writes[0].stage_migrations).toBeUndefined();
  expect(writes[0].stages.some((s: any) => s.id === "stage-1")).toBe(false);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});

test("stale editor can explicitly reload and save against the latest version", async ({
  page,
}) => {
  const { writes } = await installMocks(page, { conflictOnce: true });
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Edit Contacted", exact: true })
    .click();
  await page.getByLabel("Stage name", { exact: true }).fill("Contact made");
  await page.getByRole("button", { name: "Save stage", exact: true }).click();
  await expect(page.getByLabel("Stage name", { exact: true })).toHaveValue(
    "Contact made",
  );
  await page
    .getByRole("button", { name: "Reload editor", exact: true })
    .click();
  await expect(page.getByLabel("Stage name", { exact: true })).toHaveValue(
    "Reached out",
  );
  await page.getByLabel("Stage name", { exact: true }).fill("Contact made");
  await page.getByRole("button", { name: "Save stage", exact: true }).click();
  await expect(page.locator('[data-slot="dialog-content"]')).not.toBeVisible();
  expect(writes[1].expected_updated_at).toBe("2026-09-02T10:00:00Z");
});

test("failed background refresh preserves the open editor and its input", async ({
  page,
}) => {
  await installMocks(page, { failSave: true, failRefreshAfterSave: true });
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Edit Contacted", exact: true })
    .click();
  await page.getByLabel("Stage name", { exact: true }).fill("Keep this draft");
  await page.getByRole("button", { name: "Save stage", exact: true }).click();
  await expect(
    page.getByText(
      "Could not refresh pipelines. Showing the last loaded version.",
    ),
  ).toBeAttached();
  await expect(page.locator('[data-slot="dialog-content"]')).toBeVisible();
  await expect(page.getByLabel("Stage name", { exact: true })).toHaveValue(
    "Keep this draft",
  );
});

test("pointer dragging moves a stage directly to a distant position", async ({
  page,
}) => {
  const { writes } = await installMocks(page);
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  const start = await page
    .getByRole("button", { name: "Drag Lead found", exact: true })
    .boundingBox();
  const target = await page
    .getByRole("button", { name: "Drag Presentation scheduled", exact: true })
    .boundingBox();
  expect(start).toBeTruthy();
  expect(target).toBeTruthy();
  await page.mouse.move(
    start!.x + start!.width / 2,
    start!.y + start!.height / 2,
  );
  await page.mouse.down();
  await page.mouse.move(start!.x + start!.width / 2, start!.y + 20, {
    steps: 3,
  });
  await page.mouse.move(
    target!.x + target!.width / 2,
    target!.y + target!.height / 2,
    { steps: 12 },
  );
  await expect(
    page.getByText("Lead found is over position 5 of 7.", { exact: true }),
  ).toBeAttached();
  await page.mouse.up();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].stages[4].id).toBe("stage-0");
});

test("pipeline deal type is optional and offers new or existing business", async ({
  page,
}) => {
  const { writes } = await installMocks(page);
  await page.goto(url);
  await page
    .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Create pipeline", exact: true })
    .click();
  const dialog = page.getByRole("dialog", {
    name: "Create pipeline",
    exact: true,
  });
  await page
    .getByLabel("Pipeline name", { exact: true })
    .fill("Customer growth");
  await expect(
    page.getByRole("combobox", { name: "Default deal type" }),
  ).not.toBeVisible();
  await dialog.getByText("Optional settings", { exact: true }).click();
  const type = page.getByRole("combobox", { name: "Default deal type" });
  await expect(type).toHaveText("New business");
  await type.click();
  await expect(page.getByRole("option")).toHaveText([
    "New business",
    "Existing business",
  ]);
  await page
    .getByRole("option", { name: "Existing business", exact: true })
    .click();
  await expect(dialog).toContainText(
    "Renewals, upgrades, and additional purchases from existing customers.",
  );
  await page.screenshot({ path: "/tmp/helpin-pipeline-options-desktop.png" });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await expect(
    dialog.getByRole("button", { name: "Create pipeline", exact: true }),
  ).toBeInViewport();
  await page.screenshot({ path: "/tmp/helpin-pipeline-options-mobile.png" });
  await dialog
    .getByRole("button", { name: "Create pipeline", exact: true })
    .click();
  await expect(dialog).not.toBeVisible();
  expect(writes[0].default_commercial_motion).toBe("existing_business");
  await expect(
    page.getByRole("heading", { name: /Customer growth/ }),
  ).toBeVisible();
});

for (const motion of ["expansion", "renewal"]) {
  test(`editing pipeline details preserves its saved ${motion} classification`, async ({
    page,
  }) => {
    const { writes } = await installMocks(page, { motion });
    await page.goto(url);
    await page
      .getByRole("button", { name: "Show stages for Outbound sales pipeline", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Edit pipeline", exact: true })
      .click();
    const dialog = page.getByRole("dialog", {
      name: "Pipeline details",
      exact: true,
    });
    await dialog.getByText("Optional settings", { exact: true }).click();
    await expect(
      page.getByRole("combobox", { name: "Default deal type" }),
    ).toHaveText("Existing business");
    await page
      .getByLabel("Pipeline name", { exact: true })
      .fill("Customer renewals");
    await dialog
      .getByRole("button", { name: "Save pipeline", exact: true })
      .click();
    await expect(dialog).not.toBeVisible();
    expect(writes[0].default_commercial_motion).toBe(motion);

    await page
      .getByRole("button", { name: "Edit pipeline", exact: true })
      .click();
    await dialog.getByText("Optional settings", { exact: true }).click();
    await page.getByRole("combobox", { name: "Default deal type" }).click();
    await page
      .getByRole("option", { name: "Existing business, selected", exact: true })
      .click();
    await page
      .getByRole("dialog", { name: "Pipeline details", exact: true })
      .getByRole("button", { name: "Save pipeline", exact: true })
      .click();
    await expect(dialog).not.toBeVisible();
    expect(writes[1].default_commercial_motion).toBe("existing_business");
  });
}
