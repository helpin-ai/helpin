import { test, expect, type Page } from '@playwright/test';

const base = { workspace_id: 'ws', objective_type: 'strategic', state: 'active', health: 'on_track', position: 0, archived: false, created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z' };
const records = [
  { id: 'annual', name: 'Reach our annual growth target', planned_start_date: '2026-01-01', deadline: '2026-12-31' },
  { id: 'current', name: 'Make publishing feel effortless', planned_start_date: '2026-07-01', deadline: '2026-09-30' },
  { id: 'past', name: 'Improve trial conversion', deadline: '2026-06-30', health: 'off_track' },
  { id: 'old', name: 'Previous year objective', deadline: '2025-12-31' },
  { id: 'unscheduled', name: 'Explore a partner program' },
  { id: 'closed', name: 'Completed onboarding', deadline: '2026-09-15', state: 'closed' },
].map(item => ({ objective: { ...base, ...item }, owners: [], owner_member_ids: [], teams: [], key_results: [], labels: [], suggested_health: 'on_track',
  stats: { key_result_count: 3, key_result_avg_pct: 67, epic_count: 1, epic_progress_pct: 50 },
  epics: [{ epic: { id: `epic-${item.id}`, name: 'Publishing experience', color: '#c4b5fd', state: 'active' }, stats: { task_count: 10, done_task_count: 5 } }],
}));
async function setup(page: Page, readonly = false) {
  await page.clock.setFixedTime(new Date('2026-09-27T12:00:00Z'));
  const writes: unknown[] = [];
  await page.route('**/api/**', async route => {
    const url = new URL(route.request().url());
    let body: unknown = [];
    if (url.pathname.endsWith('/me')) body = { membership: { role: readonly ? 'viewer' : 'admin' }, permissions: readonly ? ['pm.read'] : ['pm.read', 'pm.edit'], team_memberships: [], modules: ['pm'] };
    if (url.pathname.endsWith('/objectives')) {
      if (route.request().method() === 'POST') { writes.push(route.request().postDataJSON()); body = records[0]; }
      else body = records.filter(item => !url.searchParams.get('state') || item.objective.state === url.searchParams.get('state'));
    }
    await route.fulfill({ contentType: 'application/json', body: JSON.stringify(body) });
  });
  return writes;
}

test('retains card content, groups once, reveals closed goals, and navigates to details', async ({ page }) => {
  await setup(page);
  await page.goto('/e2e/pm/harness/objectives.html');
  await expect(page.locator('[data-objective-column]')).toHaveCount(5);
  await page.screenshot({ path: '/tmp/helpin-objectives-desktop.png' });
  await expect(page.locator('[data-objective-column] > div:first-child h2')).toHaveText(['Unscheduled', 'Q4, 2026', 'Q3, 2026', 'Q2, 2026', 'Q1, 2026']);
  const annual = page.getByRole('region', { name: 'Q4, 2026', exact: true });
  await expect(annual.getByText('KR Progress')).toBeVisible();
  await expect(annual.getByText('Epic Progress')).toBeVisible();
  await expect(annual.getByText('Publishing experience')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Reach our annual growth target' })).toHaveCount(1);
  await expect(page.getByRole('link', { name: 'Completed onboarding' })).toHaveCount(0);
  await page.getByRole('button', { name: 'Closed objectives in Q3, 2026' }).click();
  await expect(page.getByRole('link', { name: 'Completed onboarding' })).toBeVisible();
  await page.getByRole('button', { name: 'Previous year' }).click();
  await expect(page.getByRole('region', { name: 'Q4, 2025', exact: true }).getByRole('link')).toHaveText('Previous year objective');
  await page.getByRole('searchbox', { name: 'Search objectives' }).fill('no matches');
  await expect(page.getByText('No objectives match these filters')).toBeVisible();
  await page.getByRole('button', { name: 'Year: 2025' }).click();
  await expect(page.getByRole('option', { name: '2026', exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await page.getByRole('searchbox', { name: 'Search objectives' }).fill('');
  await page.getByRole('link', { name: 'Previous year objective' }).click();
  await expect(page.getByRole('heading', { name: 'Objective detail destination' })).toBeVisible();
});

test('quarter creation prefills the real dialog and sends editable calendar dates', async ({ page }) => {
  const writes = await setup(page);
  await page.goto('/e2e/pm/harness/objectives.html');
  await page.getByRole('button', { name: 'Add objective to Q4, 2026' }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByText('Oct 1, 2026', { exact: true })).toBeVisible();
  await expect(dialog.getByText('Dec 31, 2026', { exact: true })).toBeVisible();
  await dialog.getByRole('textbox', { name: 'Objective title' }).fill('Quarterly test objective');
  await dialog.getByRole('button', { name: 'Create Objective', exact: true }).click();
  await expect(dialog).toHaveCount(0);
  expect(writes).toEqual([expect.objectContaining({ name: 'Quarterly test objective', planned_start_date: '2026-10-01T00:00:00Z', deadline: '2026-12-31T00:00:00Z' })]);
  await page.getByRole('button', { name: 'Add objective to Unscheduled' }).click();
  await expect(page.getByRole('dialog').getByText('Dec 31, 2026', { exact: true })).toHaveCount(0);
});

for (const theme of ['light', 'dark']) test(`mobile ${theme}: only board scrolls horizontally and current-quarter shortcut works`, async ({ page }) => {
  await setup(page, true);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/e2e/pm/harness/objectives.html?theme=${theme}`);
  await expect(page.getByRole('region', { name: 'Unscheduled', exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await expect(page.getByRole('button', { name: /^Add objective to/ })).toHaveCount(0);
  await page.getByRole('button', { name: 'This quarter', exact: true }).click();
  const inset = await page.getByRole('region', { name: 'Objectives by quarter' }).evaluate(el => parseFloat(getComputedStyle(el).paddingLeft));
  await expect.poll(() => page.getByRole('region', { name: 'Q3, 2026', exact: true }).evaluate(el => Math.round(el.getBoundingClientRect().left))).toBe(inset);
  await page.screenshot({ path: `/tmp/helpin-objectives-${theme}-mobile.png` });
});
