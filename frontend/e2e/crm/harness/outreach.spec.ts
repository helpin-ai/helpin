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
  await page.getByText("Automatic enrollment", { exact: true }).click();
  await page
    .getByRole("combobox", { name: "Enrollment stage", exact: true })
    .click();
  await page.getByRole("option", { name: "Sales · Qualified" }).click();
  await page.getByRole("combobox", { name: "Automation sender" }).click();
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
  await page.getByRole("button",{name:"Close",exact:true}).click();
  await expect(page.getByRole("alertdialog",{name:"Discard review changes?"})).toBeVisible();
  await page.getByRole("button",{name:"Keep reviewing",exact:true}).click();
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
