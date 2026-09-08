import { expect, test, type Page } from '@playwright/test';
import { installPlaybookMocks } from '../fixtures/playbooks';

const url = '/e2e/crm/harness/playbooks.html?detail';
const step = (page: Page, name: string) => page.getByRole('navigation', { name: 'Playbook setup steps' }).getByRole('button', { name: new RegExp(name) });

test('five direct steps preserve edits and save the entire draft from review without execution', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page, { draft: true });
  await page.goto(url);
  await expect(page.getByRole('navigation', { name: 'Playbook setup steps' }).getByRole('button')).toHaveCount(5);
  await expect(step(page, 'Purpose & scope')).toHaveAttribute('aria-current', 'step');
  await expect(page.getByLabel('Milestone 1', { exact: true })).not.toBeVisible();
  await page.getByLabel('Name', { exact: true }).fill('A focused customer journey');
  await page.getByRole('button', { name: 'Continue', exact: true }).click();
  await page.getByLabel('Milestone 1', { exact: true }).fill('Customer confirms the need');
  await step(page, 'Follow-up').click();
  await page.getByLabel('Check after (hours)', { exact: true }).fill('72');
  await step(page, 'Purpose & scope').click();
  await expect(page.getByLabel('Name', { exact: true })).toHaveValue('A focused customer journey');
  await step(page, 'Milestones').click();
  await expect(page.getByLabel('Milestone 1', { exact: true })).toHaveValue('Customer confirms the need');
  await step(page, 'Review & automation').click();
  await expect(page.getByRole('button', { name: 'Preview matches', exact: true })).toBeDisabled();
  expect(writes).toHaveLength(0);
  await page.getByRole('button', { name: 'Save draft', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save draft', exact: true })).toBeDisabled();
  expect(writes).toHaveLength(1);
  expect(writes[0].body).toMatchObject({ operation: 'update_draft', definition: { name: 'A focused customer journey', policy: { check_after_hours: 72 } } });
  expect(writes[0].body.definition.milestones[0].name).toBe('Customer confirms the need');
  await expect(page.getByRole('button', { name: 'Review & publish', exact: true })).toBeEnabled();
  await expect(page.getByText('Publish the playbook to connect automation.')).toBeVisible();
  await expect(page.getByText('Customer confirms the need → Next step agreed', { exact: true })).toBeVisible();
  await expect(page.getByText('Customer messages: approval required · CRM changes: approval required · Tasks: not allowed', { exact: true })).toBeVisible();
  await page.screenshot({ path: '/tmp/helpin-playbook-stepper-review.png', fullPage: true });
});

test('saving reveals an invalid field in another step and visiting does not imply completion', async ({ page }) => {
  const { item, writes } = await installPlaybookMocks(page, { draft: true });
  item.playbook.draft.objective = '';
  await page.goto(url);
  await step(page, 'Milestones').click();
  await page.getByLabel('Milestone 1', { exact: true }).fill('');
  await step(page, 'Review & automation').click();
  await page.getByRole('button', { name: 'Save draft', exact: true }).click();
  await expect(step(page, 'Milestones')).toHaveAttribute('aria-current', 'step');
  await expect(page.getByLabel('Milestone 1', { exact: true })).toBeFocused();
  expect(writes).toHaveLength(0);
  await expect(step(page, 'Purpose & scope')).not.toHaveAccessibleName(/complete/);
  await expect(step(page, 'Milestones')).not.toHaveAccessibleName(/complete/);
});

test('read-only users can navigate every step on a narrow dark view', async ({ page }) => {
  const { writes, item } = await installPlaybookMocks(page, { readOnly: true });
  item.playbook.draft.journey = 'custom';
  item.published_version!.definition.journey = 'custom';
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${url}&theme=dark`);
  await page.getByRole('tab', { name: 'Setup', exact: true }).click();
  await expect(page.getByLabel('Name', { exact: true })).toBeDisabled();
  await step(page, 'Team & permissions').click();
  await expect(page.getByRole('combobox', { name: 'Responsible owner', exact: true })).toBeDisabled();
  await step(page, 'Review & automation').click();
  await expect(step(page, 'Review & automation')).toHaveAccessibleName('Review & automation, complete');
  await expect(page.getByText('An admin with Automation access can connect this playbook.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Save draft', exact: true })).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  expect(writes).toHaveLength(0);
  await page.screenshot({ path: '/tmp/helpin-playbook-stepper-narrow.png', fullPage: true });
});

test('automation settings survive step changes without a save or implicit activation', async ({ page }) => {
  const { automation, writes } = await installPlaybookMocks(page);
  automation.connection = { id: 'connection-1', playbook_id: 'book-1', playbook_version_id: 'version-2', version: 1, fingerprint: 'reviewed', execution_enabled: false, published_by_member_id: 'member-1', published_at: new Date().toISOString() };
  await page.goto(url);
  await page.getByRole('tab', { name: 'Setup', exact: true }).click();
  await step(page, 'Review & automation').click();
  await page.getByText('Usage limits', { exact: true }).click();
  await page.getByLabel('Checks per signal / day', { exact: true }).fill('7');
  await step(page, 'Purpose & scope').click();
  await step(page, 'Review & automation').click();
  await expect(page.getByLabel('Checks per signal / day', { exact: true })).toHaveValue('7');
  expect(writes).toHaveLength(0);
  await page.getByRole('button', { name: 'Save settings', exact: true }).click();
  await expect(page.getByRole('dialog')).toContainText('No existing or paused signal starts automatically');
  expect(writes).toHaveLength(0);
});
