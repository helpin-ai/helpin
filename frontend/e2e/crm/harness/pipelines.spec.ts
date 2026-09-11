import { expect, test, type Page } from "@playwright/test";

async function installMocks(
  page: Page,
  options: {
    failSave?: boolean;
    readOnly?: boolean;
    conflictOnce?: boolean;
    failRefreshAfterSave?: boolean;
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
    default_commercial_motion: "new_business",
    deal_count: 12,
    stages,
    updated_at: "2026-09-01T10:00:00Z",
  };
  const writes: Record<string, any>[] = [];
  const list = [pipeline];
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
      return route.fulfill({ json: pipeline });
    }
    return route.fulfill({ json: {} });
  });
  return { writes, pipeline };
}

const url = "/e2e/crm/harness/pipelines.html";

test("moves a stage several positions in one action and preserves stage identities", async ({
  page,
}) => {
  const { writes } = await installMocks(page);
  await page.goto(url);
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
