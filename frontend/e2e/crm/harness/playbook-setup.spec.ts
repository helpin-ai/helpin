import { expect, test, type Page } from '@playwright/test';
import { installPlaybookMocks } from '../fixtures/playbooks';

const url = '/e2e/crm/harness/playbooks.html?detail';
const step = (page: Page, name: string) => page.getByRole('navigation', { name: 'Playbook setup steps' }).getByRole('button', { name: new RegExp(name) });
const publish = (page: Page) => page.getByRole('button', { name: 'Publish', exact: true });

test('four setup steps end at Monitoring and preserve the whole draft across Automation', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page, { draft: true });
  await page.goto(url);
  const steps = page.getByRole('navigation', { name: 'Playbook setup steps' }).getByRole('button');
  await expect(steps).toHaveCount(4);
  await expect(steps.last()).toHaveAccessibleName(/Monitoring/);
  await expect(page.getByRole('button', { name: 'Preview matches', exact: true })).toBeVisible();
  await page.getByLabel('Playbook name', { exact: true }).fill('A focused customer journey');
  await page.getByRole('button', { name: 'Continue', exact: true }).click();
  await page.getByLabel('Milestone 1', { exact: true }).fill('Customer confirms the need');
  await step(page, 'Monitoring').click();
  await page.getByLabel('Check after (hours)', { exact: true }).fill('72');
  await expect(page.getByRole('button', { name: 'Continue', exact: true })).toHaveCount(0);
  await page.getByRole('tab', { name: 'Automation', exact: true }).click();
  await expect(page.getByText('Publish the playbook to connect automation.')).toBeVisible();
  await page.getByRole('tab', { name: /^Setup/ }).click();
  await expect(page.getByLabel('Check after (hours)', { exact: true })).toHaveValue('72');
  await step(page, 'Purpose & scope').click();
  await expect(page.getByLabel('Playbook name', { exact: true })).toHaveValue('A focused customer journey');
  await expect(page.getByRole('button', { name: 'Preview matches', exact: true })).toBeDisabled();
  expect(writes).toHaveLength(0);
  await page.getByRole('button', { name: 'Save draft', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save draft', exact: true })).toBeDisabled();
  expect(writes).toHaveLength(1);
  expect(writes[0].body).toMatchObject({ operation: 'update_draft', definition: { name: 'A focused customer journey', policy: { check_after_hours: 72 } } });
  expect(writes[0].body.definition.milestones[0].name).toBe('Customer confirms the need');
  await expect(publish(page)).toBeEnabled();
});

test('blocked Publish explains live missing fields by step on hover and focus without writes', async ({ page }) => {
  const { item, writes } = await installPlaybookMocks(page, { draft: true });
  item.playbook.draft.objective = '';
  item.playbook.draft.milestones![1].success_criteria = '';
  item.playbook.draft.responsibilities.escalation_member_id = null;
  await page.goto(url);
  await expect(publish(page)).toBeDisabled();
  await expect(publish(page)).toHaveAttribute('aria-disabled', 'true');
  expect(await publish(page).evaluate((element) => (element as HTMLButtonElement).disabled)).toBe(false);
  await publish(page).hover();
  const tooltip = page.getByRole('tooltip');
  for (const text of ['Purpose & scope', 'Milestones', 'Team & permissions']) await expect(tooltip).toContainText(text);
  for (const text of [/desired outcome/i, /Milestone 2.*success criteria/i, /escalation owner/i]) await expect(tooltip).toContainText(text);
  await page.mouse.move(0, 0);
  await publish(page).focus();
  await expect(publish(page)).toBeFocused();
  await expect(tooltip).toBeVisible();
  await publish(page).click({ force: true });
  await expect(tooltip).toBeVisible();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(writes).toHaveLength(0);
  await expect(step(page, 'Purpose & scope')).toHaveAccessibleName(/, needs attention$/);
  await expect(step(page, 'Milestones')).toHaveAccessibleName(/, needs attention$/);
  await expect(step(page, 'Purpose & scope')).not.toHaveAccessibleName(/complete/);
  await page.getByLabel('Desired outcome', { exact: true }).fill('Agree the next step');
  await step(page, 'Milestones').click();
  await page.getByLabel('Success criteria', { exact: true }).nth(1).fill('The customer confirms a date.');
  await step(page, 'Team & permissions').click();
  await page.getByRole('button', { name: 'Choose escalation owner', exact: true }).click();
  await page.getByRole('option', { name: /Maya Patel/ }).click();
  await expect(publish(page)).toBeEnabled();
  expect(writes).toHaveLength(0);
});

test('dirty published playbook saves exactly once before explicit publish confirmation with its saved revision', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page);
  await page.goto(url);
  await page.getByRole('tab', { name: /^Setup/ }).click();
  await page.getByLabel('Desired outcome', { exact: true }).fill('Confirm a qualified expansion opportunity');
  await expect(publish(page)).toBeEnabled();
  await publish(page).click();
  await expect(page.getByRole('heading', { name: 'Publish playbook?', exact: true })).toBeVisible();
  expect(writes.map((write) => write.body.operation)).toEqual(['update_draft']);
  expect(writes[0].body).toMatchObject({ expected_revision: 3, definition: { objective: 'Confirm a qualified expansion opportunity' } });
  await page.getByRole('button', { name: 'Publish playbook', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(writes.map((write) => write.body.operation)).toEqual(['update_draft', 'publish']);
  expect(writes[1].body.expected_revision).toBe(4);
});

test('failed save during Publish keeps edits and retry saves once before confirmation', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page, { saveFailures: 1 });
  await page.goto(url);
  await page.getByRole('tab', { name: /^Setup/ }).click();
  await page.getByLabel('Playbook name', { exact: true }).fill('Recoverable publish edit');
  await publish(page).click();
  await expect(page.getByRole('alert')).toContainText('Temporary save failure');
  await expect(page.getByLabel('Playbook name', { exact: true })).toHaveValue('Recoverable publish edit');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(writes.map((write) => write.body.operation)).toEqual(['update_draft']);
  await publish(page).click();
  await expect(page.getByRole('heading', { name: 'Publish playbook?', exact: true })).toBeVisible();
  expect(writes.map((write) => write.body.operation)).toEqual(['update_draft', 'update_draft']);
  expect(writes[0].body.command_key).toBe(writes[1].body.command_key);
  await page.getByRole('button', { name: 'Publish playbook', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(writes[2].body).toMatchObject({ operation: 'publish', expected_revision: 4 });
});

test('unavailable escalation member verification blocks Publish with a useful explanation', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page, { draft: true });
  await page.route('**/api/**/assignable-members**', (route) => route.fulfill({ status: 503, json: { error: 'Members are unavailable' } }));
  await page.goto(url);
  await expect(publish(page)).toBeDisabled();
  await publish(page).focus();
  await expect(page.getByRole('tooltip')).toContainText(/member|escalation/i);
  await publish(page).click({ force: true });
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(writes).toHaveLength(0);
});

test('saving reveals an invalid field in another step and visiting does not imply completion', async ({ page }) => {
  const { item, writes } = await installPlaybookMocks(page, { draft: true });
  item.playbook.draft.objective = '';
  await page.goto(url);
  await step(page, 'Milestones').click();
  await page.getByLabel('Milestone 1', { exact: true }).fill('');
  await step(page, 'Monitoring').click();
  await page.getByRole('button', { name: 'Save draft', exact: true }).click();
  await expect(step(page, 'Milestones')).toHaveAttribute('aria-current', 'step');
  await expect(page.getByLabel('Milestone 1', { exact: true })).toBeFocused();
  expect(writes).toHaveLength(0);
  await expect(step(page, 'Purpose & scope')).not.toHaveAccessibleName(/complete/);
  await expect(step(page, 'Milestones')).not.toHaveAccessibleName(/complete/);
});

test('read-only users can navigate setup and Automation on a narrow dark view', async ({ page }) => {
  const { writes, item } = await installPlaybookMocks(page, { readOnly: true });
  item.playbook.draft.journey = 'custom';
  item.published_version!.definition.journey = 'custom';
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${url}&theme=dark`);
  await page.getByRole('tab', { name: /^Setup/ }).click();
  await expect(page.getByLabel('Playbook name', { exact: true })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Preview matches', exact: true })).toBeEnabled();
  await page.getByRole('button', { name: 'Preview matches', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Preview matches', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Northstar', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Apply playbook', exact: true })).toHaveCount(0);
  expect(writes).toHaveLength(0);
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await step(page, 'Team & permissions').click();
  await expect(page.getByRole('combobox', { name: 'Owner role', exact: true })).toBeDisabled();
  await page.getByRole('tab', { name: 'Automation', exact: true }).click();
  await expect(page.getByText('An admin with Automation access can connect this playbook.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Save draft', exact: true })).toHaveCount(0);
  await expect(publish(page)).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Add signals', exact: true })).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  expect(writes).toHaveLength(0);
  await page.screenshot({ path: '/tmp/helpin-playbook-stepper-narrow.png', fullPage: true });
});

test('automation settings survive tab changes without a save or implicit activation', async ({ page }) => {
  const { automation, writes } = await installPlaybookMocks(page);
  automation.connection = { id: 'connection-1', playbook_id: 'book-1', playbook_version_id: 'version-2', version: 1, fingerprint: 'reviewed', execution_enabled: false, published_by_member_id: 'member-1', published_at: new Date().toISOString() };
  await page.goto(url);
  await page.getByRole('tab', { name: 'Automation', exact: true }).click();
  await page.getByText('Usage limits', { exact: true }).click();
  await page.getByLabel('Checks per signal / day', { exact: true }).fill('7');
  await page.getByRole('tab', { name: /^Setup/ }).click();
  await step(page, 'Monitoring').click();
  await page.getByRole('tab', { name: 'Automation', exact: true }).click();
  await expect(page.getByLabel('Checks per signal / day', { exact: true })).toHaveValue('7');
  expect(writes).toHaveLength(0);
  await page.getByRole('button', { name: 'Save settings', exact: true }).click();
  await expect(page.getByRole('dialog')).toContainText('No existing or paused signal starts automatically');
  expect(writes).toHaveLength(0);
});

test.describe('touch Publish explanation', () => {
  test.use({ hasTouch: true, viewport: { width: 390, height: 844 } });
  test('tapping blocked Publish reveals missing fields without opening confirmation', async ({ page }) => {
    const { item, writes } = await installPlaybookMocks(page, { draft: true });
    item.playbook.draft.objective = '';
    await page.goto(url);
    await expect(publish(page)).toBeDisabled();
    await publish(page).tap({ force: true });
    await expect(page.getByRole('tooltip')).toContainText(/desired outcome/i);
    await expect(page.getByRole('dialog')).toHaveCount(0);
    expect(writes).toHaveLength(0);
  });
});

test('a refreshed conflicting version blocks Publish and preserves the local draft', async ({ page }) => {
  const { item, writes } = await installPlaybookMocks(page, { conflict: true });
  await page.goto(url);
  await page.getByRole('tab', { name: /^Setup/ }).click();
  await page.getByLabel('Playbook name', { exact: true }).fill('My unsaved revision');
  await publish(page).click();
  await expect(page.getByRole('alert').first()).toContainText('has changed');
  item.playbook.revision = 4;
  await page.getByRole('button', { name: 'Try again', exact: true }).first().click();
  await expect(publish(page)).toBeDisabled();
  await publish(page).focus();
  await expect(page.getByRole('tooltip')).toContainText(/changed|saved version|reload/i);
  await expect(page.getByLabel('Playbook name', { exact: true })).toHaveValue('My unsaved revision');
  await publish(page).click({ force: true });
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(writes.map((write) => write.body.operation)).toEqual(['update_draft']);
});
