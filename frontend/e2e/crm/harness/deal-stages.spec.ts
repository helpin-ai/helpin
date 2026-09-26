import { expect, test, type Page, type Route } from '@playwright/test';
test.setTimeout(120_000);
const stages = [
  {
    id: 'lead',
    name: 'Lead',
    stage_type: 'open',
    position: 0,
    probability: 20,
    color: '#788596',
  },
  {
    id: 'discussion',
    name: 'In Discussion',
    stage_type: 'open',
    position: 1,
    probability: 50,
    color: '#4e8fea',
  },
  {
    id: 'won',
    name: 'Won',
    stage_type: 'won',
    position: 2,
    probability: 100,
    color: '#45a557',
  },
].map((s) => ({ ...s, pipeline_id: 'sales' }));
async function install(page: Page, holdFirst = false) {
  const deals = [
    { id: 'a', name: 'Alpha', stage_id: 'lead', amount: 100 },
    { id: 'b', name: 'Beta', stage_id: 'lead', amount: 1200 },
  ].map((d) => ({
    ...d,
    pipeline_id: 'sales',
    workspace_id: 'ws-deals',
    display_id: d.id,
    currency: 'USD',
    revenue_type: 'monthly',
    custom_properties: {},
    stage: stages[0],
  }));
  const writes: Record<string, unknown>[] = [];
  let held: Route | undefined;
  await page.route('**/api/**', (route) => {
    const req = route.request(),
      url = new URL(req.url()),
      path = url.pathname;
    if (path.endsWith('/assignable-members'))
      return route.fulfill({
        json: [
          {
            id: 'member-1',
            status: 'active',
            display_name: 'Alex Sales',
            email: 'alex@example.test',
          },
        ],
      });
    if (path.endsWith('/pipelines'))
      return route.fulfill({
        json: [{ id: 'sales', name: 'Sales', is_default: true, stages }],
      });
    if (path.includes('/deals/') && req.method() === 'PUT') {
      const id = path.split('/').at(-1),
        body = req.postDataJSON();
      writes.push({ id, ...body });
      if (holdFirst && id === 'a') {
        held = route;
        return;
      }
      const deal = deals.find((d) => d.id === id)!;
      Object.assign(deal, body, {
        stage: stages.find((s) => s.id === body.stage_id) ?? deal.stage,
      });
      return route.fulfill({ json: deal });
    }
    if (path.endsWith('/deals')) {
      let rows = deals;
      if (url.searchParams.has('filters')) {
        const group = JSON.parse(url.searchParams.get('filters')!);
        if (group.rules.some((r: { field: string }) => r.field === 'amount'))
          rows = deals.filter((d) => d.amount >= 1000);
      }
      return route.fulfill({ json: { data: rows, total: rows.length } });
    }
    return route.fulfill({ json: [] });
  });
  await page.goto('/e2e/crm/harness/deal-creation.html?page');
  await expect(page.locator('[data-deal-id="a"]')).toBeVisible();
  return {
    writes,
    failHeld: async () => {
      expect(held).toBeTruthy();
      await held!.fulfill({ status: 500, json: { error: 'Move failed' } });
    },
  };
}
async function drag(page: Page, id: string, target: string) {
  const source = await page.locator(`[data-deal-id="${id}"]`).boundingBox();
  const destination = await page
    .locator(`[data-stage-id="${target}"]`)
    .boundingBox();
  expect(source).toBeTruthy();
  expect(destination).toBeTruthy();
  await page.mouse.move(source!.x + 80, source!.y + 35);
  await page.mouse.down();
  await page.mouse.move(source!.x + 90, source!.y + 40, { steps: 3 });
  await page.mouse.move(destination!.x + 150, destination!.y + 200, {
    steps: 15,
  });
  await page.mouse.up();
}
test('failed drag rolls back only that deal while another move succeeds', async ({
  page,
}) => {
  const state = await install(page, true);
  await drag(page, 'a', 'discussion');
  await expect.poll(() => state.writes.length).toBe(1);
  await expect(
    page.locator('[data-stage-id="discussion"] [data-deal-id="a"]'),
  ).toBeVisible();
  await drag(page, 'b', 'won');
  await expect.poll(() => state.writes.length).toBe(2);
  await expect(
    page.locator('[data-stage-id="won"] [data-deal-id="b"]'),
  ).toBeVisible();
  await state.failHeld();
  await expect(
    page.locator('[data-stage-id="lead"] [data-deal-id="a"]'),
  ).toBeVisible();
  await expect(
    page.locator('[data-stage-id="won"] [data-deal-id="b"]'),
  ).toBeVisible();
  await expect(page.getByText('Move failed', { exact: true })).toBeVisible();
});
test('task-style controls, colored stage picker and display toggles work in list view', async ({
  page,
}) => {
  const state = await install(page);
  const list = page.getByRole('button', { name: 'List view', exact: true });
  expect(await list.getAttribute('class')).toContain(
    'text-muted-foreground/60',
  );
  const display = page.getByRole('button', {
    name: 'Display settings',
    exact: true,
  });
  const listBox = await list.boundingBox(),
    displayBox = await display.boundingBox();
  expect(displayBox!.x).toBeGreaterThan(listBox!.x);
  await list.click();
  await expect(list).toHaveAttribute('aria-pressed', 'true');
  await page
    .locator('[data-deal-id="a"]')
    .getByRole('button', { name: 'Stage', exact: true })
    .click();
  await expect(
    page
      .getByRole('option', { name: 'Won', exact: true })
      .locator('[data-testid="state-color-dot"]'),
  ).toHaveCSS('background-color', 'rgb(69, 165, 87)');
  await page.getByRole('option', { name: 'Won', exact: true }).click();
  await expect.poll(() => state.writes.length).toBe(1);
  expect(state.writes[0]).toMatchObject({ id: 'a', stage_id: 'won' });
  await display.click();
  await expect(page.getByRole('option', { name: /Amount/ })).toBeVisible();
  await page.getByRole('option', { name: /Amount/ }).click();
  await expect(page.getByRole('option', { name: /Amount/ })).toHaveAttribute(
    'data-checked',
    'false',
  );
  await page.screenshot({ path: '/tmp/helpin-deal-display-settings.png' });
});
test('deal filters use contact query controls and removable chips', async ({
  page,
}) => {
  await install(page);
  await page.getByRole('button', { name: /Filters/ }).click();
  const dialog = page.getByRole('dialog').last();
  await dialog.getByRole('combobox', { name: 'Filter 1 field' }).click();
  await page.getByRole('option', { name: 'Amount', exact: true }).click();
  await dialog.getByRole('combobox', { name: 'Filter 1 operator' }).click();
  await page.getByRole('option', { name: 'is at least', exact: true }).click();
  await page.getByRole('spinbutton', { name: 'Amount value' }).fill('1000');
  await page
    .getByRole('button', { name: 'Apply filters', exact: true })
    .click();
  await expect(
    page.getByRole('button', { name: 'Remove Amount filter' }),
  ).toBeVisible();
  await expect(page.locator('[data-deal-id="a"]')).toHaveCount(0);
  await page.getByRole('button', { name: 'Remove Amount filter' }).click();
  await expect(page.locator('[data-deal-id="a"]')).toBeVisible();
});

test('failed owner edit reports the error and releases the deal for retry', async ({
  page,
}) => {
  const state = await install(page, true);
  const card = page.locator('[data-deal-id="a"]');
  await card.locator('button').first().click();
  await page.getByRole('option', { name: /Alex Sales/ }).click();
  await expect.poll(() => state.writes.length).toBe(1);
  await expect(card).toHaveAttribute('aria-busy', 'true');
  await state.failHeld();
  await expect(page.getByText('Move failed', { exact: true })).toBeVisible();
  await expect(card).toHaveAttribute('aria-busy', 'false');
});


for (const dropKey of ['Space', 'Enter']) test(`keyboard stage moves and cancelled drags preserve deal state (${dropKey})`, async ({page}) => {
  const state = await install(page);
  const card = page.locator('section [data-deal-id="a"]');
  await card.focus();
  await page.keyboard.press('Space');
  await expect(card).toHaveAttribute('aria-pressed', 'true');
  await page.keyboard.press('ArrowRight');
  await expect(page.getByRole('status')).toContainText('over droppable area discussion');
  await page.keyboard.press('Escape');
  await expect(page.locator('[data-stage-id="lead"] [data-deal-id="a"]')).toBeVisible();
  expect(state.writes).toHaveLength(0);
  await expect(page.locator('[data-deal-id="a"][inert]')).toHaveCount(0);
  await card.focus();
  await page.keyboard.press('Space');
  await expect(card).toHaveAttribute('aria-pressed', 'true');
  await page.keyboard.press('ArrowRight');
  await expect(page.getByRole('status')).toContainText('over droppable area discussion');
  await page.keyboard.press(dropKey);
  await expect.poll(() => state.writes.length).toBe(1);
  expect(state.writes[0]).toMatchObject({id:'a',stage_id:'discussion'});
  await expect(page.locator('[data-stage-id="discussion"] [data-deal-id="a"]')).toBeVisible();
});

test('display settings fit narrow dark screens and restore after reopening', async ({page}) => {
  await page.setViewportSize({width:390,height:844});
  await install(page);
  await page.evaluate(() => document.documentElement.classList.add('dark'));
  await page.getByRole('button', {name:'Display settings',exact:true}).click();
  const option = page.getByRole('option', {name:/^Amount(?:, selected)?$/});
  await option.click();
  await expect(option).toHaveAttribute('data-checked','false');
  await page.keyboard.press('Escape');
  await page.getByRole('button', {name:'Display settings',exact:true}).click();
  await expect(option).toHaveAttribute('data-checked','false');
  const box = await page.locator('[data-dropdown-content]').last().boundingBox();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(390);
  await page.screenshot({path:'/tmp/crm-display-narrow-dark.png'});
});
