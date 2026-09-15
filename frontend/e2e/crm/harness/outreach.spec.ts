import { test, expect, type Page } from "@playwright/test";
test.setTimeout(120_000);
test.use({ actionTimeout: 15000 });
const date = "2026-09-14T10:00:00Z";
const template = {
  id: "template",
  workspace_id: "ws-email",
  owner_id: "owner",
  name: "After a demo",
  subject: "Next steps, {{first_name|there}}",
  body_html:
    "<p>Hi {{first_name|there}},</p><p>Thanks for your time today. Here is the next step we discussed.</p>",
  shared: true,
  version: 1,
  created_at: date,
  updated_at: date,
};
const sequence = {
  id: "sequence",
  workspace_id: "ws-email",
  owner_id: "owner",
  name: "Demo follow-up",
  status: "active",
  version: 1,
  steps: [
    {
      kind: "email",
      mode: "automatic",
      delay_days: 0,
      subject: "Thanks, {{first_name}}",
      body_html:
        "<p>Hi {{first_name}},</p><p>It was great to meet your team.</p>",
    },
    {
      kind: "email",
      mode: "review",
      delay_days: 3,
      subject: "Any questions?",
      body_html: "<p>Happy to help with next steps.</p>",
    },
  ],
  timezone: "UTC",
  start_hour: 9,
  end_hour: 17,
  weekdays: true,
  include_signature: true,
  entry_stage_id: "",
  entry_account_id: "",
  created_at: date,
  updated_at: date,
};
async function setup(page: Page) {
  let templates = [{ ...template }];
  let sequences = [{ ...sequence }];
  let recipients: any[] = [];
  const requests: any[] = [];
  await page.route("**/api/**", async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    const path = url.pathname;
    let data: any = [];
    const payload =
      method === "POST" || method === "PUT"
        ? route.request().postDataJSON()
        : undefined;
    requests.push({ path, method, payload });
    if (path.endsWith("/templates")) {
      if (payload) {
        templates.push({ ...template, ...payload, id: "new-template" });
        data = templates.at(-1);
      } else data = templates;
    } else if (path.endsWith("/sequences")) {
      if (payload) {
        const row = { ...sequence, ...payload, id: "new-sequence", version: 1 };
        sequences.push(row);
        data = row;
      } else data = sequences;
    } else if (path.includes("/sequences/") && path.endsWith("/enroll")) {
      if (url.searchParams.has("preview"))
        data = [
          {
            contact_id: "contact",
            contact_name: "Amna",
            email: "amna@example.com",
            steps: sequence.steps.map((s) => ({
              ...s,
              subject: s.subject.replace("{{first_name}}", "Amna"),
              body_html: s.body_html.replace("{{first_name}}", "Amna"),
            })),
          },
        ];
      else {
        recipients = [
          {
            id: "recipient",
            sequence_id: "sequence",
            sequence_name: sequence.name,
            contact_id: "contact",
            contact_name: "Amna",
            email: "amna@example.com",
            owner_id: "owner",
            account_id: "mailbox",
            status: "needs_review",
            step_index: 1,
            steps: sequence.steps,
            next_at: date,
            updated_at: date,
          },
        ];
        data = recipients;
      }
    } else if (path.includes("/sequences/")) {
      data = { ...sequence, ...payload, version: 2 };
      sequences = [data];
    } else if (path.endsWith("/enrollments")) data = recipients;
    else if (path.includes("/enrollments/")) {
      if (payload) {
        recipients[0] = {
          ...recipients[0],
          status: payload.action === "approve" ? "active" : "paused",
        };
        data = { updated: true };
      } else data = { enrollment: recipients[0], deliveries: [] };
    } else if (path.endsWith("/email/accounts"))
      data = [
        {
          id: "mailbox",
          member_id: "owner",
          email_address: "waqar@contentstudio.io",
          can_send: true,
          is_active: true,
          status: "connected",
          provider: "gmail",
        },
      ];
    else if (path.endsWith("/contacts"))
      data = {
        data: [
          { id: "contact", first_name: "Amna", email: "amna@example.com" },
        ],
        total: 1,
        page: 1,
        per_page: 30,
      };
    else if (path.endsWith("/pipelines"))
      data = [
        {
          id: "pipeline",
          name: "Sales",
          stages: [{ id: "stage", name: "Qualified", stage_type: "open" }],
        },
        {
          id: "renewals",
          name: "Renewals",
          stages: [
            { id: "renewal-stage", name: "Review renewal", stage_type: "open" },
            { id: "renewal-won", name: "Renewed", stage_type: "won" },
          ],
        },
      ];
    else if (path.endsWith("/me"))
      data = {
        membership: { role: "owner", status: "active" },
        permissions: ["crm.read", "crm.edit", "pm.read", "pm.edit"],
        team_memberships: [{ team_id: "team", role: "owner" }],
      };
    else if (path.includes("/settings"))
      data = {
        teams: [{ id: "team", name: "Sales" }],
        people: [],
        memberships: [],
        user_memberships: [],
      };
    await route.fulfill({ json: data });
  });
  return requests;
}
test("template library creates and previews reusable email", async ({
  page,
}) => {
  await setup(page);
  await page.goto("/e2e/crm/harness/outreach.html");
  await page.getByRole("button", { name: "After a demo" }).click();
  await expect(
    page
      .frameLocator("iframe")
      .getByText("Thanks for your time today.", { exact: false }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Close", exact: true }).click();
  await page.getByRole("button", { name: "New template", exact: true }).click();
  await page
    .getByRole("textbox", { name: "Template name" })
    .fill("Renewal reminder");
  await page
    .getByRole("textbox", { name: "Template subject" })
    .fill("Your renewal");
  await page
    .getByRole("button", { name: "Save template", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Renewal reminder" }),
  ).toBeVisible();
});
test("sequence editor adds task and configures stage enrollment", async ({
  page,
}) => {
  const requests = await setup(page);
  await page.goto("/e2e/crm/harness/outreach.html?tab=sequences");
  await page.getByRole("button", { name: "Demo follow-up" }).click();
  await page.getByRole("button", { name: "+ Task step", exact: true }).click();
  await page
    .getByRole("textbox", { name: "Task title" })
    .fill("Call {{full_name}}");
  await page.getByText("Enrollment", { exact: true }).click();
  await page
    .getByRole("combobox", { name: "Enrollment pipeline", exact: true })
    .click();
  await page.getByRole("option", { name: "Sales", exact: true }).click();
  await page
    .getByRole("combobox", { name: "Enrollment stage", exact: true })
    .click();
  await page.getByRole("option", { name: "Qualified", exact: true }).click();
  await page.getByRole("combobox", { name: "Sending mailbox" }).click();
  await page.getByRole("option", { name: "waqar@contentstudio.io" }).click();
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect
    .poll(
      () =>
        requests.filter(
          (r) => r.method === "PUT" && r.path.includes("/sequences/"),
        ).length,
    )
    .toBe(1);
  const save = requests.find(
    (r) => r.method === "PUT" && r.path.includes("/sequences/"),
  );
  expect(save.payload.steps[2].kind).toBe("task");
  expect(save.payload.entry_stage_id).toBe("stage");
});
test("enrollment preview and edited review approval", async ({ page }) => {
  const requests = await setup(page);
  await page.goto("/e2e/crm/harness/outreach.html");
  await page.getByRole("button", { name: "activity", exact: true }).click();
  await page.getByRole("button", { name: "Add contacts", exact: true }).click();
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "Review emails" }).click();
  await expect(page.getByRole("dialog")).toContainText("Thanks, Amna");
  await page.getByRole("button", { name: "Start sequence for 1" }).click();
  await page.getByRole("button", { name: "Amna amna@example.com" }).click();
  await page
    .getByRole("textbox", { name: "Review subject" })
    .fill("Edited follow-up");
  await page.getByRole("button", { name: "Close", exact: true }).click();
  await expect(
    page.getByRole("alertdialog", { name: "Discard review changes?" }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Keep reviewing", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Approve email", exact: true })
    .click();
  await expect
    .poll(
      () =>
        requests.find((r) => r.payload?.action === "approve")?.payload.subject,
    )
    .toBe("Edited follow-up");
});
for (const mode of ["light", "dark", "narrow"])
  test(`sequence visual ${mode}`, async ({ page }) => {
    await setup(page);
    if (mode === "narrow")
      await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(
      `/e2e/crm/harness/outreach.html?tab=sequences${mode === "dark" ? "&dark" : ""}`,
    );
    await page.getByRole("button", { name: "Demo follow-up" }).click();
    await expect(
      page.getByRole("textbox", { name: "Step 1 subject" }),
    ).toBeVisible();
    await expect(page.locator("[contenteditable=true]")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Add contacts", exact: true }),
    ).toBeEnabled();
    await page.screenshot({
      path: `/tmp/crm-outreach-${mode}.png`,
      fullPage: true,
    });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBeTruthy();
  });

test("public unsubscribe works without signing in and only submits on confirmation", async ({
  page,
}) => {
  const token =
    "12345678-1234-1234-1234-123456789abc12345678-1234-1234-1234-123456789abc";
  let submitted = 0;
  await page.route("**/api/**", (route) =>
    route.fulfill({ status: 401, json: { error: "Not signed in" } }),
  );
  await page.route(`**/api/crm/outreach/unsubscribe/${token}`, (route) => {
    if (route.request().method() === "POST") submitted++;
    return route.fulfill({
      contentType: "text/html; charset=utf-8",
      body: "<h1>You’re unsubscribed</h1>",
    });
  });
  await page.goto(`/email-preferences/${token}`);
  await expect(
    page.getByRole("heading", { name: "Stop follow-up emails?" }),
  ).toBeVisible();
  expect(submitted).toBe(0);
  await page.getByRole("button", { name: "Unsubscribe", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "You’re unsubscribed" }),
  ).toBeVisible();
  expect(submitted).toBe(1);
});
test("starter sequence requires context and opens as an unpublished editable draft", async ({
  page,
}) => {
  const requests = await setup(page);
  await page.goto("/e2e/crm/harness/outreach.html?tab=sequences");
  await page.getByRole("button", { name: "Browse starters" }).click();
  const use = page.getByRole("button", { name: "Use sequence starter" });
  await expect(use).toBeDisabled();
  await page
    .getByRole("textbox", { name: "Your company", exact: true })
    .fill("ContentStudio");
  await page
    .getByRole("textbox", { name: "Topic", exact: true })
    .fill("campaign approvals");
  await page
    .getByRole("textbox", { name: "How you help", exact: true })
    .fill("reduce manual coordination");
  await page
    .getByRole("textbox", { name: "Useful follow-up insight", exact: true })
    .fill("Agree on one reviewer before the campaign starts.");
  await expect(use).toBeEnabled();
  await expect(
    page.frameLocator("iframe").first().locator("body"),
  ).toContainText("ContentStudio");
  await page.screenshot({
    path: "/tmp/crm-starters-light.png",
    fullPage: true,
  });
  await use.click();
  await expect(
    page.getByRole("textbox", { name: "Sequence name" }),
  ).toHaveValue("Relevant cold introduction");
  await expect(
    page.getByRole("button", { name: "Save draft", exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByRole("combobox", { name: "Email delivery mode" }),
  ).toContainText("Review before sending");
  expect(requests.filter((r) => r.method === "POST")).toHaveLength(0);
  await page.getByRole("button", { name: "Save draft", exact: true }).click();
  await expect
    .poll(
      () =>
        requests.find(
          (r) => r.method === "POST" && r.path.endsWith("/sequences"),
        )?.payload.status,
    )
    .toBe("draft");
});
test("email starters filter by scenario and create a private editable template", async ({
  page,
}) => {
  await setup(page);
  await page.goto("/e2e/crm/harness/outreach.html");
  await page.getByRole("button", { name: "Browse starters" }).click();
  await page
    .getByRole("searchbox", { name: "Search scenarios" })
    .fill("missed");
  await expect(
    page.getByRole("heading", { name: "Reschedule a missed meeting" }),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "Topic", exact: true })
    .fill("onboarding");
  await page
    .getByRole("textbox", { name: "How to reschedule", exact: true })
    .fill("reply with a time that works for you");
  await page.getByRole("button", { name: "Use email starter" }).click();
  await expect(
    page.getByRole("textbox", { name: "Template subject" }),
  ).toHaveValue("Another time to connect?");
  await expect(page.getByRole("checkbox")).not.toBeChecked();
  await page.getByRole("button", { name: "Close", exact: true }).click();
  await expect(
    page.getByRole("alertdialog", { name: "Discard template changes?" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Keep editing", exact: true }).click();
});
for (const mode of ["dark", "narrow"])
  test(`starter library visual ${mode}`, async ({ page }) => {
    await setup(page);
    if (mode === "narrow")
      await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(
      `/e2e/crm/harness/outreach.html?${mode === "dark" ? "dark" : ""}`,
    );
    await page.getByRole("button", { name: "Browse starters" }).click();
    await expect(
      page.getByRole("heading", { name: "Email starters" }),
    ).toBeVisible();
    await page.screenshot({
      path: `/tmp/crm-starters-${mode}.png`,
      fullPage: true,
    });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
  });

test("activity shows an empty placeholder when there are no recipients", async ({
  page,
}) => {
  await setup(page);
  await page.goto("/e2e/crm/harness/outreach.html");
  await page.getByRole("button", { name: "activity", exact: true }).click();
  await expect(
    page.getByText("No activity yet", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Loading recipients…")).not.toBeVisible();
});

test("activity errors offer retry and recover to the empty placeholder", async ({
  page,
}) => {
  await setup(page);
  let failed = true;
  await page.route("**/api/crm/outreach/enrollments?**", (route) =>
    route.fulfill({
      status: failed ? 503 : 200,
      json: failed ? { error: "Service unavailable" } : [],
    }),
  );
  await page.goto("/e2e/crm/harness/outreach.html");
  await page.getByRole("button", { name: "activity", exact: true }).click();
  await expect(
    page.getByText("Couldn’t load activity", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("No activity yet", { exact: true }),
  ).not.toBeVisible();
  failed = false;
  await page.getByRole("button", { name: "Try again", exact: true }).click();
  await expect(
    page.getByText("No activity yet", { exact: true }),
  ).toBeVisible();
});

test("activity stops loading when the recipients request stalls", async ({
  page,
}) => {
  await setup(page);
  await page.route("**/api/crm/outreach/enrollments?**", () => {});
  await page.goto("/e2e/crm/harness/outreach.html");
  await page.getByRole("button", { name: "activity", exact: true }).click();
  await expect(page.getByText("Loading recipients…")).toBeVisible();
  await expect(
    page.getByText("Couldn’t load activity", { exact: true }),
  ).toBeVisible({ timeout: 22000 });
  await expect(page.getByText("Loading recipients…")).not.toBeVisible();
});

test("automatic enrollment scopes stages to the selected pipeline and clears old selections", async ({
  page,
}) => {
  await setup(page);
  await page.goto("/e2e/crm/harness/outreach.html?tab=sequences");
  await page.getByRole("button", { name: "Demo follow-up" }).click();
  await page.getByText("Enrollment", { exact: true }).click();
  const pipeline = page.getByRole("combobox", {
    name: "Enrollment pipeline",
    exact: true,
  });
  const stage = page.getByRole("combobox", {
    name: "Enrollment stage",
    exact: true,
  });
  await expect(stage).not.toBeVisible();
  await pipeline.click();
  await page.getByRole("option", { name: "Sales", exact: true }).click();
  await stage.click();
  await expect(
    page.getByRole("option", { name: "Review renewal", exact: true }),
  ).not.toBeVisible();
  await page.getByRole("option", { name: "Qualified", exact: true }).click();
  await pipeline.click();
  await page.getByRole("option", { name: "Renewals", exact: true }).click();
  await expect(stage).not.toContainText("Qualified");
  await stage.click();
  await expect(
    page.getByRole("option", { name: "Qualified", exact: true }),
  ).not.toBeVisible();
  await expect(
    page.getByRole("option", { name: "Renewed", exact: true }),
  ).not.toBeVisible();
  await page
    .getByRole("option", { name: "Review renewal", exact: true })
    .click();
  await pipeline.click();
  await page
    .getByRole("option", { name: "Manual enrollment only", exact: true })
    .click();
  await expect(stage).not.toBeVisible();
  await expect(
    page.getByRole("combobox", { name: "Sending mailbox" }),
  ).not.toBeVisible();
});

test("automatic enrollment restores the pipeline for a saved stage", async ({
  page,
}) => {
  await setup(page);
  await page.route("**/api/crm/outreach/sequences?**", (route) =>
    route.fulfill({
      json: [
        {
          ...sequence,
          entry_stage_id: "renewal-stage",
          entry_account_id: "mailbox",
        },
      ],
    }),
  );
  await page.goto("/e2e/crm/harness/outreach.html?tab=sequences");
  await page.getByRole("button", { name: "Demo follow-up" }).click();
  await page.getByText("Enrollment", { exact: true }).click();
  await expect(
    page.getByRole("combobox", { name: "Enrollment pipeline", exact: true }),
  ).toContainText("Renewals");
  await expect(
    page.getByRole("combobox", { name: "Enrollment stage", exact: true }),
  ).toContainText("Review renewal");
  await page
    .getByRole("combobox", { name: "Enrollment stage", exact: true })
    .click();
  await expect(page.locator("aside summary").nth(0)).toHaveText("Enrollment");
  await expect(page.locator("aside summary").nth(1)).toHaveText("Delivery");
  await page.screenshot({
    path: "/tmp/crm-enrollment-pipeline-stages.png",
    fullPage: true,
  });
});

test("sequence steps can collapse and expand with the keyboard", async ({
  page,
}) => {
  await setup(page);
  await page.goto("/e2e/crm/harness/outreach.html?tab=sequences");
  await page.getByRole("button", { name: "Demo follow-up" }).click();
  const step = page.getByRole("button", { name: /1 Email Start immediately/ });
  await expect(step).toHaveAttribute("aria-expanded", "true");
  await expect(
    page.getByRole("textbox", { name: "Step 1 subject" }),
  ).toBeVisible();
  await step.focus();
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("textbox", { name: "Step 1 subject" }),
  ).not.toBeVisible();
  const closed = page.getByRole("button", {
    name: /1 Thanks,.*Start immediately/,
  });
  await expect(closed).toHaveAttribute("aria-expanded", "false");
  await closed.focus();
  await page.keyboard.press("Enter");
  await expect(step).toHaveAttribute("aria-expanded", "true");
});

test("enrollment sender has a placeholder before choosing a mailbox", async ({
  page,
}) => {
  await setup(page);
  await page.route("**/api/crm/outreach/sequences?**", (route) =>
    route.fulfill({ json: [{ ...sequence, entry_stage_id: "stage" }] }),
  );
  await page.goto("/e2e/crm/harness/outreach.html?tab=sequences");
  await page.getByRole("button", { name: "Demo follow-up" }).click();
  await page.getByText("Enrollment", { exact: true }).click();
  await expect(
    page.getByRole("combobox", { name: "Sending mailbox", exact: true }),
  ).toContainText("Select a mailbox");
  await expect(page.getByText(/Saving authorizes this rule/)).not.toBeVisible();
});

test("enrollment without an eligible mailbox shows a connection prompt instead of an empty dropdown", async ({
  page,
}) => {
  await setup(page);
  await page.route("**/api/crm/outreach/sequences?**", (route) =>
    route.fulfill({ json: [{ ...sequence, entry_stage_id: "stage" }] }),
  );
  await page.route("**/api/crm/email/accounts?**", (route) =>
    route.fulfill({ json: [] }),
  );
  await page.goto("/e2e/crm/harness/outreach.html?tab=sequences");
  await page.getByRole("button", { name: "Demo follow-up" }).click();
  await page.getByText("Enrollment", { exact: true }).click();
  await expect(
    page.getByText("No sending mailbox connected.", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("combobox", { name: /mailbox|sender/i }),
  ).not.toBeVisible();
  await expect(
    page.getByRole("link", { name: "Connect a mailbox" }),
  ).toHaveAttribute("href", "/w/email-test/settings/crm-email");
  await expect(page.getByText(/Saving authorizes this rule/)).not.toBeVisible();
});

for (const mode of ["light", "dark", "narrow"])
test(`activity shows mailbox capacity and saves owner sending limits ${mode}`, async ({
  page,
}) => {
  await setup(page);
  let limit = 100;
  await page.route("**/api/crm/outreach/mailbox-capacity**", async (route) => {
    if (route.request().method() === "PUT")
      limit = route.request().postDataJSON().daily_limit;
    await route.fulfill({
      json: [
        {
          account_id: "mailbox",
          email: "waqar@contentstudio.io",
          daily_limit: limit,
          manual_reserve: 10,
          min_interval_seconds: 60,
          used: 60,
          sent: 59,
          remaining: limit - 60,
          sequence_remaining: limit - 70,
          queued: 45,
        },
      ],
    });
  });
  if (mode === "narrow") await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/e2e/crm/harness/outreach.html${mode === "dark" ? "?dark" : ""}`);
  await page.getByRole("button", { name: "activity", exact: true }).click();
  await expect(page.getByText("45 queued", { exact: true })).toBeVisible();
  await page.screenshot({ path: `/tmp/crm-capacity-${mode}.png`, fullPage: true });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
  await page
    .getByRole("button", { name: "Sending limits for waqar@contentstudio.io" })
    .click();
  await page.getByRole("spinbutton", { name: "Daily email limit" }).fill("5");
  await expect(page.getByRole("button", { name: "Save limits", exact: true })).toBeDisabled();
  await page.getByRole("spinbutton", { name: "Daily email limit" }).fill("150");
  await page.screenshot({ path: `/tmp/crm-capacity-limits-${mode}.png`, fullPage: true });
  await page.getByRole("button", { name: "Save limits", exact: true }).click();
  await expect.poll(() => limit).toBe(150);
});

for (const saved of [false, true]) {
  test(`sequence preserves draft after empty save response (${saved ? 'existing' : 'new'})`, async ({ page }) => {
    await setup(page);
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto('/e2e/crm/harness/outreach.html?tab=sequences');
    await page.getByRole('button', { name: saved ? /Demo follow-up/ : 'New sequence' }).click();
    await page.getByRole('textbox', { name: 'Sequence name', exact: true }).fill('Draft to preserve');
    await page.getByRole('textbox', { name: 'Step 1 subject', exact: true }).fill('Keep this subject');
    await page.route('**/api/crm/outreach/sequences**', async route => {
      if (['POST', 'PUT'].includes(route.request().method())) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: 'null' });
      } else await route.fallback();
    });
    await page.getByRole('button', { name: saved ? 'Save changes' : 'Save draft', exact: true }).click();
    await expect(page.getByText('Could not confirm the saved sequence. Your changes are still in the editor.', { exact: true })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Sequence name', exact: true })).toHaveValue('Draft to preserve');
    await expect(page.getByRole('textbox', { name: 'Step 1 subject', exact: true })).toHaveValue('Keep this subject');
    await expect(page.getByRole('button', { name: saved ? 'Save changes' : 'Save draft', exact: true })).toBeEnabled();
    expect(errors).toEqual([]);
    await expect(page.getByText('Sequence saved', { exact: true })).toHaveCount(0);
  });
}

test('activity rejects an empty recipient response instead of staying loading', async ({ page }) => {
  await setup(page);
  await page.route('**/api/crm/outreach/enrollments?**', route => route.fulfill({status:200,contentType:'application/json',body:'null'}));
  await page.goto('/e2e/crm/harness/outreach.html');
  await page.getByRole('button', { name:'activity', exact:true }).click();
  await expect(page.getByText('Couldn’t load activity', {exact:true})).toBeVisible();
  await expect(page.getByText('Loading recipients…', {exact:true})).toHaveCount(0);
});

test('activity explains how to enable sending limits without a connected mailbox', async ({ page }) => {
  await setup(page);
  await page.route('**/api/crm/outreach/mailbox-capacity?**', route => route.fulfill({json:[]}));
  await page.goto('/e2e/crm/harness/outreach.html');
  await page.getByRole('button', { name:'activity', exact:true }).click();
  await expect(page.getByRole('link', {name:'Connect a sending mailbox',exact:true})).toHaveAttribute('href','/w/email-test/settings/crm-email');
});
