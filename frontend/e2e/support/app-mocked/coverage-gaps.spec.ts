import { expect, test } from "@playwright/test";
import { COVERAGE_GAP, installCoverageMocks } from "../fixtures/coverageE2E";

test.beforeEach(async ({ page }) => {
  test.setTimeout(180_000);
  await page.setViewportSize({ width: 1440, height: 1000 });
});

async function openGap(page: import("@playwright/test").Page) {
  await page.goto("/w/workspace/support/coverage", {
    waitUntil: "domcontentloaded",
  });
  await page
    .getByRole("button", { name: new RegExp(COVERAGE_GAP.title) })
    .click({ timeout: 120_000 });
  await expect(
    page.getByRole("heading", { name: COVERAGE_GAP.title }),
  ).toBeVisible();
  await expect(page.locator("header").filter({ has: page.getByRole("heading", { name: COVERAGE_GAP.title }) })).toContainText("Open");
  await page.addStyleTag({
    content: ".helpin-query-devtools { display: none !important; }",
  });
}

test("gap starts with the reason and one action, with sources on demand", async ({
  page,
}, testInfo) => {
  const fixture = await installCoverageMocks(page);
  await openGap(page);
  await expect(
    page.locator("[data-agent-context-message]").getByText(COVERAGE_GAP.analysis_explanation.ai_failure, {
      exact: false,
    }),
  ).toBeVisible();
  await expect(
    page.getByText(COVERAGE_GAP.impact_explanation + ".", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Prepare a fix", exact: true }),
  ).toBeVisible();
  const findings = page.locator("[data-agent-context-message]");
  await expect(findings.getByText("Customer need:", { exact: true })).toBeVisible();
  await expect(findings.getByText(COVERAGE_GAP.analysis_explanation.human_resolution, { exact: false })).toBeVisible();
  await expect(findings.getByRole("link", { name: /How can I correct the billing address/ })).toHaveAttribute("href", "/w/workspace/support/conv-1");
  await expect(findings.getByRole("link", { name: "Billing overview", exact: true }).first()).toHaveAttribute("href", "/w/workspace/docs/documents/doc-billing");
  await expect(page.getByText("View sources (9)", { exact: true })).toHaveCount(0);
  await page.screenshot({
    path: testInfo.outputPath("coverage-desktop.png"),
    fullPage: true,
  });
  const sourcesControl = findings.locator("summary").filter({ hasText: /^Sources/ });
  await sourcesControl.focus();
  await sourcesControl.press("Enter");
  await expect(
    page.getByText("Showing the latest 1 of 9 evidence records."),
  ).toBeVisible();
  await expect(findings.getByRole("paragraph").filter({ hasText: COVERAGE_GAP.evidence[0].excerpt })).toBeVisible();
  await findings.getByText("Analysis details", { exact: true }).click();
  await expect(findings.getByText(COVERAGE_GAP.analysis_explanation.decision_reason, { exact: true })).toBeVisible();
  await expect(page.getByText("Analysis details", { exact: true })).toHaveCount(1);
  await page.screenshot({ path: testInfo.outputPath("coverage-sources.png"), fullPage: true, animations: "disabled" });
  await findings.getByText("Sources", { exact: true }).click();
  await findings.getByText("Analysis details", { exact: true }).click();
  await page
    .getByRole("button", { name: "Prepare a fix", exact: true })
    .click();
  await expect(
    page.locator("[data-coverage-conversation] textarea"),
  ).not.toHaveValue("");
  await page.locator("[data-coverage-conversation] textarea").press("Enter");
  await expect(
    page.getByText("Billing owns the invoice correction process.", {
      exact: false,
    }),
  ).toBeVisible();
  await expect(page.locator("[data-coverage-conversation]")).toBeVisible();
  expect(fixture.started()).toBe(1);
  expect(fixture.created).toHaveLength(1);
  expect(fixture.created[0].coverage_gap_id).toBe("gap-1");
  expect(fixture.sent[0].page_context).toMatchObject({
    entity_type: "support_coverage_gap",
    entity_id: "gap-1",
  });
  await page.getByRole("button", { name: "Close gap details" }).click();
  await page
    .getByRole("button", { name: new RegExp(COVERAGE_GAP.title) })
    .click();
  await expect(
    page.getByText("Billing owns the invoice correction process.", {
      exact: false,
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Prepare a fix", exact: true }),
  ).toHaveCount(0);
  expect(fixture.started()).toBe(1);
  await page
    .locator("[data-coverage-conversation] textarea")
    .fill("Why is the existing guidance insufficient?");
  await page.locator("[data-coverage-conversation] textarea").press("Enter");
  await expect.poll(() => fixture.sent.length).toBe(2);
  expect(fixture.created).toHaveLength(1);
  expect(fixture.sent[1].page_context).toMatchObject({
    entity_type: "support_coverage_gap",
    entity_id: "gap-1",
  });
});

test("reviewer edits the proposed fix and saves it without closing the gap", async ({
  page,
}, testInfo) => {
  const fixture = await installCoverageMocks(page, { proposal: true });
  await openGap(page);
  await expect(
    page.getByRole("button", { name: "Prepare a fix", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Edit draft", exact: true }).click();
  await page
    .getByRole("textbox", { name: "Article title", exact: true })
    .fill("Invoice correction requests");
  const editor = page.locator(
    '[data-coverage-proposal-editor] [contenteditable="true"]',
  );
  await expect(editor).toBeVisible();
  await editor
    .locator("p")
    .first()
    .fill(
      "Contact support with the invoice number. Billing will review the request.",
    );
  await page.screenshot({
    path: testInfo.outputPath("coverage-review.png"),
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Save draft for review", exact: true })
    .click();
  await expect(
    page.getByRole("link", { name: "Review saved draft" }),
  ).toBeVisible();
  expect(fixture.applied).toHaveLength(1);
  expect(fixture.applied[0].title).toBe("Invoice correction requests");
  expect(JSON.stringify(fixture.applied[0].content)).toContain(
    "invoice number",
  );
  expect(fixture.gap.status).toBe("open");
  await expect(
    page.locator("header").filter({ has: page.getByRole("heading", { name: COVERAGE_GAP.title }) }).getByText("Open", { exact: true }),
  ).toBeVisible();
});

test("read-only gaps stay readable on narrow screens in dark mode", async ({
  page,
}, testInfo) => {
  await installCoverageMocks(page, { readOnly: true, existingRun: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await openGap(page);
  await page.evaluate(() => document.documentElement.classList.add("dark"));
  await expect(
    page.getByText("Billing owns the invoice correction process.", {
      exact: false,
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Mark resolved", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.locator("[data-coverage-conversation] textarea"),
  ).toHaveCount(0);
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth > window.innerWidth,
  );
  expect(overflow).toBe(false);
  await page.screenshot({
    path: testInfo.outputPath("coverage-mobile-dark.png"),
    fullPage: true,
  });
  const findings = page.locator("[data-agent-context-message]");
  await findings.locator("summary").filter({ hasText: "Saved findings" }).click();
  await findings.getByText("Sources", { exact: true }).click();
  await findings.getByText("Analysis details", { exact: true }).click();
  await expect(findings.getByRole("paragraph").filter({ hasText: COVERAGE_GAP.evidence[0].excerpt })).toBeVisible();
  await expect(findings.getByText(COVERAGE_GAP.analysis_explanation.decision_reason, { exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)).toBe(false);
  await page.screenshot({ path: testInfo.outputPath("coverage-sources-mobile-dark.png"), fullPage: true, animations: "disabled" });
});

test("a failed conversation load offers retry and prevents a duplicate start", async ({
  page,
}) => {
  await installCoverageMocks(page, { runError: true });
  await openGap(page);
  await expect(
    page.getByRole("button", { name: "Retry", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Prepare a fix", exact: true }),
  ).toHaveCount(0);
});

test("assistant asks for one missing fact and continues in the same gap", async ({
  page,
}, testInfo) => {
  const fixture = await installCoverageMocks(page, {
    existingRun: true,
    question: true,
  });
  await openGap(page);
  await expect(
    page.getByRole("dialog", {
      name: "Agent needs your response",
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    page.getByText("Who reviews invoice corrections?", { exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("coverage-question.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: /Billing team/ }).click();
  await page
    .getByRole("button", { name: "Submit answers", exact: true })
    .click();
  await expect(
    page.getByRole("dialog", {
      name: "Agent needs your response",
      exact: true,
    }),
  ).toHaveCount(0);
  expect(fixture.answers).toHaveLength(1);
  expect(JSON.stringify(fixture.answers[0])).toContain("Billing team");
  expect(fixture.started()).toBe(0);
});

test("completed assistant work opens the exact prepared proposal for review", async ({
  page,
}) => {
  await installCoverageMocks(page, { existingRun: true, prepared: true });
  await openGap(page);
  await expect(
    page.getByRole("link", { name: "Review prepared fix", exact: true }),
  ).toHaveAttribute(
    "href",
    "/w/workspace/docs/documents/doc-ready?proposal=change-ready",
  );
  await expect(
    page.getByText("The gap stays open until you verify the fix.", {
      exact: false,
    }),
  ).toBeVisible();
});

test("full-editor handoff saves the reviewed draft and keeps resolution separate", async ({
  page,
}) => {
  const fixture = await installCoverageMocks(page, { proposal: true });
  await openGap(page);
  await page
    .getByRole("button", { name: "Save and open editor", exact: true })
    .click();
  await expect(page).toHaveURL(/\/w\/workspace\/docs\/documents\/saved-doc$/);
  expect(fixture.applied).toHaveLength(1);
  expect(fixture.gap.status).toBe("open");
});
