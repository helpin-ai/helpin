import { expect, test } from '@playwright/test';
import { installPlaybookMocks } from '../fixtures/playbooks';

const harness = '/e2e/crm/harness/playbooks.html';

test('scannable list, shared filters, template creation, and no execution writes', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page);
  await page.goto(harness);
  await expect(page.getByRole('heading', { name: 'Playbooks', exact: true })).toBeVisible();
  await expect(page.getByRole('columnheader')).toHaveText(['Playbook', 'Status', 'Open signals', 'Paused', 'Closed']);
  await page.getByRole('button', { name: 'About signal counts' }).hover();
  await expect(page.getByRole('tooltip')).toContainText('not unique customers');
  await page.getByLabel('Search playbooks', { exact: true }).fill('not a match');
  await expect(page.getByText('No matching playbooks', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Clear filters', exact: true }).first().click();
  await page.getByRole('button', { name: /^Playbook status:/ }).click();
  await page.getByRole('option', { name: 'Draft', exact: true }).click();
  await expect(page.getByText('No matching playbooks', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Remove Playbook status filter' })).toBeVisible();
  await expect(page.getByRole('option')).toHaveCount(0);
  await page.getByRole('button', { name: 'Draft', exact: true }).click();
  await page.getByRole('option', { name: 'Accepting signals', exact: true }).click();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('button', { name: /^Playbook status:/ })).toHaveText('Accepting signals');
  await expect(page.getByRole('table', { name: 'Playbooks', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Remove Playbook status filter' }).click();
  await expect(page.getByRole('button', { name: /^Playbook status:/ })).toHaveText('All statuses');
  await expect(page.getByRole('button', { name: 'Clear all', exact: true })).toHaveCount(0);
  await page.screenshot({ path: '/tmp/helpin-playbooks-list.png', animations: 'disabled' });
  await page.getByRole('button', { name: 'Create playbook', exact: true }).click();
  await expect(page.getByRole('dialog')).toContainText('Nothing runs');
  await page.getByRole('button', { name: /Sales-to-success handoff/ }).click();
  await expect(page.getByRole('tab', { name: 'Setup', exact: true })).toHaveAttribute('data-state', 'active');
  expect(writes).toHaveLength(1);
  expect(writes[0].path).toBe('/crm/playbooks');
  expect(writes[0].body.creation_key).toBeTruthy();
});

test('saved draft, explicit publishing, preserved edits across tabs and safe navigation', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page);
  await page.goto(`${harness}?detail`);
  await page.getByRole('tab', { name: 'Setup', exact: true }).click();
  await page.getByLabel('Desired outcome', { exact: true }).fill('Confirm a qualified expansion opportunity');
  await page.getByRole('tab', { name: 'Signals', exact: true }).click();
  await expect(page.getByRole('table', { name: 'Playbook signals' })).toBeVisible();
  await page.getByRole('tab', { name: /Setup/ }).click();
  await expect(page.getByLabel('Desired outcome', { exact: true })).toHaveValue('Confirm a qualified expansion opportunity');
  await page.getByRole('link', { name: 'Playbooks', exact: true }).click();
  await expect(page.getByRole('dialog')).toContainText('Discard unsaved changes?');
  await page.getByRole('button', { name: 'Keep editing' }).click();
  await page.getByRole('button', { name: 'Save draft', exact: true }).click();
  await expect(page.getByText(/Unpublished changes/)).toBeVisible();
  expect(writes[0].body.operation).toBe('update_draft');
  expect(writes[0].body.expected_revision).toBe(3);
  await page.getByRole('button', { name: 'Publish', exact: true }).click();
  expect(writes).toHaveLength(1);
  await expect(page.getByRole('dialog')).toContainText('Existing signals keep their current version');
  await page.getByRole('button', { name: 'Publish playbook', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(writes.map((write) => write.body.operation)).toEqual(['update_draft', 'publish']);
  await page.screenshot({ path: '/tmp/helpin-playbooks-setup.png', animations: 'disabled' });
});

test('publish validation names missing essentials in the tooltip without an API write', async ({ page }) => {
  const { item, writes } = await installPlaybookMocks(page, { draft: true });
  item.playbook.draft.objective = '';
  item.playbook.draft.responsibilities.escalation_member_id = null;
  await page.goto(`${harness}?detail`);
  const publish = page.getByRole('button', { name: 'Publish', exact: true });
  await expect(publish).toBeDisabled();
  await publish.focus();
  await expect(page.getByRole('tooltip')).toContainText(/desired outcome/i);
  await expect(page.getByRole('tooltip')).toContainText(/escalation owner/i);
  await publish.click({ force: true });
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(writes).toHaveLength(0);
});

test('shared condition builder preserves any-match rules, and tooltips do not submit the form', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page);
  await page.goto(`${harness}?detail`);
  await page.getByRole('tab', { name: 'Setup', exact: true }).click();
  await page.getByLabel('Playbook name', { exact: true }).fill('A changed name');
  await page.getByRole('button', { name: 'About desired outcome', exact: true }).focus();
  await expect(page.getByRole('tooltip')).toContainText('The result you want for the customer');
  expect(writes).toHaveLength(0);
  await page.getByRole('button', { name: 'Add conditions', exact: true }).click();
  await page.getByRole('combobox', { name: 'Match conditions', exact: true }).click();
  await page.getByRole('option', { name: 'Match any condition', exact: true }).click();
  await page.getByPlaceholder('example.com', { exact: true }).fill('harbor.test');
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
  await expect(page.getByText('Match any', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: /Conditions 1/ }).click();
  await expect(page.getByRole('combobox', { name: 'Match conditions', exact: true })).toContainText('Match any condition');
  await expect(page.getByPlaceholder('example.com', { exact: true })).toHaveValue('harbor.test');
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: 'Save draft', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save draft', exact: true })).toBeDisabled();
  expect(writes[0].body.definition.eligibility.filter).toEqual({ logic: 'or', rules: [{ field: 'company_domain', operator: 'is', value: 'harbor.test' }] });
});

test('preview is read-only; enrollment requires confirmation and exact published version', async ({ page }) => {
  const { writes, reads } = await installPlaybookMocks(page);
  await page.goto(`${harness}?detail`);
  await page.getByRole('tab', { name: 'Setup', exact: true }).click();
  await page.getByRole('button', { name: 'Preview matches', exact: true }).click();
  await page.getByRole('button', { name: 'Northstar', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Apply playbook', exact: true })).toHaveCount(0);
  expect(writes).toHaveLength(0);
  await page.keyboard.press('Escape');
  await page.keyboard.press('Escape');
  await page.getByRole('tab', { name: 'Automation', exact: true }).click();
  await page.getByRole('button', { name: 'Add signals', exact: true }).click();
  await page.getByRole('button', { name: 'Northstar', exact: true }).click();
  await expect(page.getByRole('dialog').last()).toContainText('No messages, tasks, Flows, or Agents will start.');
  expect(writes).toHaveLength(0);
  await page.getByRole('button', { name: 'Apply playbook', exact: true }).click();
  await expect(page.getByText('No matching signals', { exact: true })).toBeVisible();
  expect(writes).toHaveLength(1);
  expect(writes[0].body).toMatchObject({ confirmed: true, version_id: 'version-2', expected_playbook_revision: 3, expected_situation_revision: 4, situation_id: 'signal-2' });
  expect(reads.some((url) => url.includes('version_id=version-2'))).toBe(true);
});

test('milestones use the pinned version, including pagination, and do not close the signal', async ({ page }) => {
  const { writes, reads } = await installPlaybookMocks(page);
  await page.goto(`${harness}?detail`);
  await page.getByRole('button', { name: 'Harbor Labs', exact: true }).click();
  await expect(page.getByText('Original customer commitment', { exact: true })).toBeVisible();
  await expect(page.getByText('Intent confirmed', { exact: true })).not.toBeVisible();
  expect(reads.some((url) => url.includes('/versions?') && url.includes('before=2'))).toBe(true);
  await page.screenshot({ path: '/tmp/helpin-playbooks-progress.png', animations: 'disabled' });
  await page.getByRole('button', { name: 'Record progress', exact: true }).click();
  await page.getByRole('combobox', { name: 'Milestone status', exact: true }).click();
  await page.getByRole('option', { name: 'Achieved', exact: true }).click();
  await page.getByLabel('What supports this assessment?', { exact: true }).fill('James confirmed the timeline in our call.');
  await page.getByRole('button', { name: 'Record progress', exact: true }).last().click();
  await expect(page.getByRole('button', { name: 'Update assessment', exact: true })).toBeVisible();
  expect(writes).toHaveLength(1);
  expect(writes[0].body).toMatchObject({ milestone_key: 'legacy', expected_revision: 4, status: 'achieved' });
  await expect(page.getByRole('button', { name: 'Record outcome', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible();
});

test('stopping enrollment leaves active work untouched', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page);
  await page.goto(`${harness}?detail`);
  await page.getByRole('tab', { name: 'Automation', exact: true }).click();
  await page.getByRole('button', { name: 'Stop new enrollment', exact: true }).click();
  await expect(page.getByRole('dialog')).toContainText('Existing signals will not be paused or closed');
  expect(writes).toHaveLength(0);
  await page.getByRole('button', { name: 'Stop enrollment', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Allow new enrollment', exact: true })).toBeVisible();
  expect(writes).toHaveLength(1);
  expect(writes[0].body).toMatchObject({ operation: 'set_enrollment', accepting_customers: false });
});

test('responsibility changes preserve the playbook; lifecycle actions require explicit reasons', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page);
  await page.goto(`${harness}?detail`);
  await page.getByRole('button', { name: 'Harbor Labs', exact: true }).click();
  await page.getByRole('button', { name: 'Edit', exact: true }).click();
  await page.getByLabel('Next step', { exact: true }).fill('Schedule the buying-team call');
  await page.getByRole('button', { name: 'Choose signal owner', exact: true }).click();
  await page.getByRole('option', { name: /Jordan Reed/ }).click();
  await page.getByRole('button', { name: 'Save changes', exact: true }).click();
  await expect(page.getByText('Schedule the buying-team call', { exact: true }).last()).toBeVisible();
  expect(writes[0].body).toMatchObject({ operation: 'update', expected_revision: 4, changes: { owner: { member_id: 'member-2' }, next_step: 'Schedule the buying-team call' } });
  expect(writes[0].body.changes).not.toHaveProperty('playbook_version_id');
  expect(writes[0].body.changes).not.toHaveProperty('checkpoint');
  await page.getByRole('button', { name: 'Pause', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Pause signal', exact: true })).toBeDisabled();
  await page.getByLabel('Reason for pausing', { exact: true }).fill('The buying team is away this week.');
  await page.getByRole('button', { name: 'Pause signal', exact: true }).click();
  await page.getByRole('button', { name: 'Resume', exact: true }).click();
  await page.getByRole('button', { name: 'Resume signal', exact: true }).click();
  await page.getByRole('button', { name: 'Record outcome', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Record and close', exact: true })).toBeDisabled();
  await page.getByLabel('What happened?', { exact: true }).fill('The customer agreed to a qualified evaluation.');
  await page.getByRole('button', { name: 'Record and close', exact: true }).click();
  await expect(page.getByText('Recorded outcome', { exact: true })).toBeVisible();
  expect(writes.map((write) => write.body.operation)).toEqual(['update', 'pause', 'resume', 'close']);
});

test('failed saves keep edits and retry the same command key', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page, { saveFailures: 1 });
  await page.goto(`${harness}?detail`);
  await page.getByRole('tab', { name: 'Setup', exact: true }).click();
  await page.getByLabel('Playbook name', { exact: true }).fill('Recoverable playbook name');
  await page.getByRole('button', { name: 'Save draft', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Temporary save failure');
  await expect(page.getByLabel('Playbook name', { exact: true })).toHaveValue('Recoverable playbook name');
  await page.getByRole('button', { name: 'Save draft', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save draft', exact: true })).toBeDisabled();
  expect(writes).toHaveLength(2);
  expect(writes[0].body.command_key).toBe(writes[1].body.command_key);
});

test('revision conflicts preserve unsaved text', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page, { conflict: true });
  await page.goto(`${harness}?detail`);
  await page.getByRole('tab', { name: 'Setup', exact: true }).click();
  await page.getByLabel('Playbook name', { exact: true }).fill('My unsaved edit');
  await page.getByRole('button', { name: 'Save draft', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('has changed');
  await expect(page.getByLabel('Playbook name', { exact: true })).toHaveValue('My unsaved edit');
  expect(writes[0].body.expected_revision).toBe(3);
});

test('read-only members can inspect without mutation controls', async ({ page }) => {
  const { writes } = await installPlaybookMocks(page, { readOnly: true });
  await page.goto(`${harness}?detail&theme=dark`);
  await expect(page.getByRole('tab', { name: 'Signals', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Add signals', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Harbor Labs', exact: true }).click();
  await expect(page.getByText('Original customer commitment', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Record progress', exact: true })).toHaveCount(0);
  await page.keyboard.press('Escape');
  await page.getByRole('tab', { name: 'Setup', exact: true }).click();
  await expect(page.getByLabel('Playbook name', { exact: true })).toBeDisabled();
  await page.screenshot({ path: '/tmp/helpin-playbooks-dark.png', animations: 'disabled' });
  expect(writes).toHaveLength(0);
});

test('empty, error and narrow views are useful working states', async ({ page }) => {
  await installPlaybookMocks(page, { empty: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(harness);
  await expect(page.getByText('Give your team a repeatable way forward')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: '/tmp/helpin-playbooks-narrow.png', animations: 'disabled' });
  await page.unroute('**/api/**');
  await installPlaybookMocks(page, { failure: true });
  await page.reload();
  await expect(page.getByRole('alert')).toContainText('could not be loaded');
  await expect(page.getByRole('button', { name: 'Try again', exact: true })).toBeVisible();
});
