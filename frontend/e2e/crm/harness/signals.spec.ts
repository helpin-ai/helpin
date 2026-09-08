import { test, expect } from '@playwright/test';
import { installSignalMocks } from '../fixtures/signals';

const url = '/e2e/crm/harness/playbooks.html?signals=1';
test('existing evidence and unassigned approvals appear by default without automation setup', async ({ page }) => {
  const mock = await installSignalMocks(page, { evidenceOnly: true, standalone: true });
  await page.goto(url);
  await expect(page.getByRole('button', { name: 'Customer requested the Inbox add-on', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Review a standalone recommendation', exact: true })).toBeVisible();
  await expect(page.getByRole('combobox', { name: 'Assignment', exact: true })).toHaveText('Everyone');
  await page.screenshot({ path: '/tmp/helpin-restored-signals.png', fullPage: true });
  await page.getByRole('button', { name: 'Customer requested the Inbox add-on', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Customer signal' })).toBeVisible();
  await page.getByText('Evidence (1)', { exact: true }).click();
  await expect(page.getByText('Could you send the monthly and annual pricing for the Inbox add-on?')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Open email' })).toHaveAttribute('href', /thread.*email-thread-1/);
  await page.screenshot({ path: '/tmp/helpin-restored-signal-drawer.png', fullPage: true });
  expect(mock.reads.some((read) => read.includes(`/signal-inbox/evidence/${mock.evidenceGroup.id}`))).toBeTruthy();
  expect(mock.writes).toHaveLength(0);
  for (const name of ['Approve', 'Apply playbook', 'Record outcome', 'Pause']) await expect(page.getByRole('button', { name, exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Mark evidence reviewed' }).click();
  await expect(page.getByText('Evidence reviewed', { exact: true }).last()).toBeVisible();
  expect(mock.writes).toEqual([{ path: '/crm/signals/evidence-1/review', body: {} }]);
});

test('personal assignment remains explicit and clearing it restores unassigned signals', async ({ page }) => {
  const mock = await installSignalMocks(page, { evidenceOnly: true });
  await page.goto(`${url}&scope=mine`);
  await expect(page.getByText('No signals match this view')).toBeVisible();
  await page.getByRole('button', { name: 'Show all signals' }).click();
  await expect(page.getByRole('button', { name: 'Customer requested the Inbox add-on', exact: true })).toBeVisible();
  expect(mock.reads.some((read) => read.includes('scope=mine'))).toBeTruthy();
  expect(mock.reads.some((read) => read.includes('scope=all'))).toBeTruthy();
  expect(mock.writes).toHaveLength(0);
});

test('read-only evidence deep links show unavailable groups without creating replacement work', async ({ page }) => {
  const mock = await installSignalMocks(page, { evidenceOnly: true, readOnly: true, groupUnavailable: true });
  await page.goto(`${url}&group=${mock.evidenceGroup.id}`);
  await expect(page.getByRole('heading', { name: 'Customer signal' })).toBeVisible();
  await expect(page.getByText('This item could not be found.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Mark evidence reviewed' })).toHaveCount(0);
  expect(mock.writes).toHaveLength(0);
});

test('scannable table, one category strip, and visible assignment/status filters use server queries', async ({ page }) => {
  const mock = await installSignalMocks(page, { unclassified: true });
  await page.goto(url);
  await expect(page.getByRole('table', { name: 'Signals' })).toBeVisible();
  await expect(page.getByRole('tablist')).toHaveCount(1);
  for (const name of ['Signal', 'Customer', 'Category', 'Owner', 'Priority']) await expect(page.getByRole('columnheader', { name, exact: name !== 'Priority' })).toBeVisible();
  await expect(page.getByText('Included in All.', { exact: false })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Confirm the customer relationship' })).toBeVisible();
  await page.screenshot({ path: '/tmp/helpin-signals-list.png', fullPage: true });
  await page.getByRole('tab', { name: 'Expansion' }).click();
  await expect(page.getByRole('button', { name: 'Team is approaching its seat limit' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Pricing requested for a 40-seat rollout' })).toHaveCount(0);
  await page.getByRole('combobox', { name: 'Assignment', exact: true }).click();
  await page.getByRole('option', { name: 'My teams' }).click();
  await expect.poll(() => mock.reads.some((read) => read.includes('scope=my_teams') && read.includes('category=expansion'))).toBeTruthy();
  expect(mock.writes).toHaveLength(0);
  await expect(page.getByRole('link', { name: 'Review queue' })).toHaveCount(0);
  await expect(page.getByRole('link', { name: 'Evidence', exact: true })).toHaveAttribute('href', /view.*evidence/);
});

test('focused drawer shows exact evidence and requires revision-bound explicit approval', async ({ page }) => {
  const mock = await installSignalMocks(page);
  await page.goto(`${url}&signal=signal-1`);
  await expect(page.getByRole('heading', { name: 'Pricing requested for a 40-seat rollout' })).toBeVisible();
  await page.screenshot({ path: '/tmp/helpin-signals-drawer.png', fullPage: true });
  await page.getByText('Evidence (1)', { exact: true }).click();
  await expect(page.getByRole('link', { name: 'Open email' })).toHaveAttribute('href', /thread.*email-thread-1/);
  await page.getByText('Activity', { exact: true }).click();
  await expect(page.getByText('Signal created', { exact: true })).toBeVisible();
  expect(mock.writes).toHaveLength(0);
  await page.getByRole('button', { name: 'Review action', exact: true }).click();
  await expect(page.getByText('Records your approval only.', { exact: false })).toBeVisible();
  await page.getByRole('button', { name: 'Approve', exact: true }).click();
  await expect(page.getByText('Approved — follow-through needed', { exact: true })).toBeVisible();
  expect(mock.writes).toHaveLength(1);
  expect(mock.writes[0].body).toEqual({ revision: 'shown-revision' });
});

test('all dismissal reasons remain available and dismissal does not close the signal', async ({ page }) => {
  const mock = await installSignalMocks(page);
  await page.goto(`${url}&signal=signal-1`);
  await page.getByRole('button', { name: 'Dismiss', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Dismiss recommendation', exact: true })).toBeDisabled();
  await page.getByRole('combobox', { name: 'Dismissal reason' }).click();
  await expect(page.getByRole('option')).toHaveCount(7);
  await page.getByRole('option', { name: 'Wrong person or account' }).click();
  await page.getByRole('button', { name: 'Dismiss recommendation', exact: true }).click();
  await expect(page.getByText('Past decisions (1)', { exact: true })).toBeVisible();
  expect(mock.writes[0].body).toEqual({ revision: 'shown-revision', reason: 'wrong_entity' });
  expect(mock.signal.situation.lifecycle).toBe('open');
});

test('a stale approval cannot be replayed after conflict', async ({ page }) => {
  const mock = await installSignalMocks(page, { conflict: true });
  await page.goto(`${url}&signal=signal-1`);
  await page.getByRole('button', { name: 'Review action', exact: true }).click();
  await page.getByRole('button', { name: 'Approve', exact: true }).click();
  await expect(page.getByText('This action has changed. Reload it before deciding.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Approve', exact: true })).toBeDisabled();
  expect(mock.writes).toHaveLength(1);
});

for (const type of ['deal_create', 'deal_advance'] as const) test(`${type} resolves actual targets and states its immediate effect`, async ({ page }) => {
  const mock = await installSignalMocks(page, { actionType: type, actionFailure: type === 'deal_advance' });
  await page.goto(`${url}&signal=signal-1`);
  await page.getByRole('button', { name: 'Review action', exact: true }).click();
  await expect(page.getByText('Sales pipeline', { exact: true })).toBeVisible();
  await expect(page.getByText(type === 'deal_create' ? 'Discovery' : 'Proposal', { exact: true })).toBeVisible();
  await expect(page.getByText(type === 'deal_create' ? 'This immediately creates the deal below.' : 'This immediately changes the deal’s stage.')).toBeVisible();
  await page.getByRole('button', { name: type === 'deal_create' ? 'Create deal' : 'Change stage', exact: true }).click();
  await expect(page.getByText(type === 'deal_create' ? 'Past decisions (1)' : 'Action failed', { exact: true }).last()).toBeVisible();
  expect(mock.writes).toHaveLength(1);
});

test('ownership and next-step edits use canonical commands without launching automation', async ({ page }) => {
  const mock = await installSignalMocks(page);
  await page.goto(`${url}&signal=signal-1`);
  await page.getByRole('button', { name: 'Edit', exact: true }).click();
  await page.getByRole('textbox', { name: 'Next step', exact: true }).fill('Ask James to confirm the budget review date');
  await page.getByRole('button', { name: 'Save changes', exact: true }).click();
  await expect(page.getByText('Ask James to confirm the budget review date', { exact: true }).last()).toBeVisible();
  expect(mock.writes).toHaveLength(1);
  expect(mock.writes[0].body).toMatchObject({ operation: 'update', expected_revision: 4, changes: { next_step: 'Ask James to confirm the budget review date' } });
  expect(Object.keys(mock.writes[0].body.changes)).toEqual(['next_step']);
});

test('read-only users can inspect but cannot decide, edit, or close work', async ({ page }) => {
  const mock = await installSignalMocks(page, { readOnly: true });
  await page.goto(`${url}&signal=signal-1`);
  await expect(page.getByText('A teammate with CRM edit access can review this action.')).toBeVisible();
  for (const name of ['Review action', 'Dismiss', 'Edit', 'Pause', 'Record outcome']) await expect(page.getByRole('button', { name, exact: true })).toHaveCount(0);
  expect(mock.writes).toHaveLength(0);
});

test('waiting work requires a checkpoint and schedules only the explicit check', async ({ page }) => {
  await page.clock.setFixedTime(new Date('2026-09-07T12:00:00Z'));
  const mock = await installSignalMocks(page);
  await page.goto(`${url}&signal=signal-1`);
  await page.getByRole('button', { name: 'Edit', exact: true }).click();
  await page.getByRole('combobox', { name: 'Follow-up status', exact: true }).click();
  await page.getByRole('option', { name: 'Waiting on customer', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save changes', exact: true })).toBeDisabled();
  await page.getByRole('button', { name: 'Choose a date', exact: true }).click();
  await page.getByRole('option', { name: /Tomorrow/ }).click();
  await page.getByLabel('Next check time', { exact: true }).fill('14:30');
  await page.getByRole('button', { name: 'Save changes', exact: true }).click();
  await expect.poll(() => mock.writes.length).toBe(1);
  expect(mock.writes[0].body.changes).toEqual({ attention: 'waiting_customer', checkpoint: { at: '2026-09-08T14:30:00.000Z' } });
});

test('pagination retains global category counts and preserves the selected category on drawer return', async ({ page }) => {
  const mock = await installSignalMocks(page);
  for (let index = 0; index < 30; index++) mock.records.push({ ...structuredClone(mock.signal), situation: { ...structuredClone(mock.signal.situation), id: `extra-${index}`, title: `Customer objective ${index}` } });
  await page.goto(url);
  await expect(page.getByRole('tab', { name: 'Sales 31', exact: true })).toBeVisible();
  await expect(page.getByRole('row')).toHaveCount(26);
  await page.getByRole('button', { name: 'Next', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Customer objective 29', exact: true })).toBeVisible();
  await page.getByRole('tab', { name: 'Expansion', exact: false }).click();
  await page.getByRole('button', { name: 'Team is approaching its seat limit', exact: true }).click();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('tab', { name: 'Expansion', exact: false })).toHaveAttribute('data-state', 'active');
  await expect(page.getByRole('row')).toHaveCount(2);
  expect(mock.writes).toHaveLength(0);
});

test('empty and failed queues remain distinguishable and recoverable', async ({ page }) => {
  await installSignalMocks(page, { empty: true });
  await page.goto(url);
  await expect(page.getByText('No signals match this view', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Show all signals' }).click();
  await expect(page.getByText('No signals yet', { exact: true })).toBeVisible();
  await installSignalMocks(page, { failure: true });
  await page.reload();
  await expect(page.getByText('Signals could not be loaded.', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Try again', exact: true })).toBeVisible();
});

test('dark and narrow views keep all table columns accessible', async ({ page }) => {
  await installSignalMocks(page);
  await page.goto(`${url}&theme=dark`);
  await expect(page.getByRole('table', { name: 'Signals' })).toBeVisible();
  await page.screenshot({ path: '/tmp/helpin-signals-dark.png', fullPage: true });
  await page.setViewportSize({ width: 640, height: 900 });
  await expect(page.getByRole('columnheader', { name: 'Priority', exact: false })).toBeAttached();
  await expect(page.getByRole('combobox', { name: 'Assignment', exact: true })).toBeVisible();
  await page.screenshot({ path: '/tmp/helpin-signals-narrow.png', fullPage: true });
});

test('old Review URL opens all pending recommendations, including unassigned standalone work', async ({ page }) => {
  const mock = await installSignalMocks(page, { standalone: true });
  await page.goto('/e2e/crm/harness/playbooks.html?review=1');
  await expect(page.getByRole('combobox', { name: 'Assignment', exact: true })).toHaveText('Everyone');
  await expect(page.getByRole('combobox', { name: 'Signal status', exact: true })).toHaveText('Needs approval');
  await expect(page.getByRole('button', { name: 'Review a standalone recommendation', exact: true })).toBeVisible();
  await expect(page.getByRole('cell', { name: 'Not scored', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Review a standalone recommendation', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Review recommendation', exact: true })).toBeVisible();
  await page.getByText('Evidence', { exact: true }).last().click();
  await expect(page.getByText('No linked source evidence is available for this signal.')).toBeVisible();
  expect(mock.writes).toHaveLength(0);
  await page.getByRole('button', { name: 'Review action', exact: true }).click();
  await page.getByRole('button', { name: 'Approve', exact: true }).click();
  await expect(page.getByText('Approved — follow-through needed', { exact: true })).toBeVisible();
  expect(mock.writes[0]).toEqual({ path: '/crm/signal-inbox/recommendations/action-1/accept', body: { revision: 'shown-revision' } });
  await page.keyboard.press('Escape');
  await expect(page.getByRole('button', { name: 'Review a standalone recommendation', exact: true })).toHaveCount(0);
});

test('priority, evidence and recommendation type stay visible and compose server filters', async ({ page }) => {
  const mock = await installSignalMocks(page, { standalone: true, actionType: 'enrichment' });
  await page.goto(`${url}&scope=all&state=needs_approval`);
  for (const name of ['Priority', 'Evidence review', 'Recommendation type', 'Sort signals']) await expect(page.getByRole('combobox', { name, exact: true })).toBeVisible();
  await page.getByRole('combobox', { name: 'Priority', exact: true }).click();
  await page.getByRole('option', { name: 'Not scored', exact: true }).click();
  await page.getByRole('combobox', { name: 'Evidence review', exact: true }).click();
  await page.getByRole('option', { name: 'No current evidence', exact: true }).click();
  await page.getByRole('combobox', { name: 'Recommendation type', exact: true }).click();
  await expect(page.getByRole('option')).toHaveCount(6);
  await page.getByRole('option', { name: 'Enrichment', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Review a standalone recommendation', exact: true })).toBeVisible();
  await expect.poll(() => mock.reads.some((read) => {
    const filter = JSON.parse(new URL(read).searchParams.get('filter') || '{}');
    return filter.rules?.length === 3 && filter.rules.some((rule: { field: string }) => rule.field === 'has_enrichment');
  })).toBeTruthy();
  await page.getByRole('button', { name: 'Review a standalone recommendation', exact: true }).click();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('combobox', { name: 'Priority', exact: true })).toHaveText('Not scored');
  await expect(page.getByRole('combobox', { name: 'Recommendation type', exact: true })).toHaveText('Enrichment');
  expect(mock.writes).toHaveLength(0);
});

test('pending approval remains visible when another action failed', async ({ page }) => {
  const mock = await installSignalMocks(page);
  mock.signal.effective_attention = 'automation_failed'; mock.signal.failed_action_count = 1;
  await page.goto(`${url}&state=needs_approval&scope=all`);
  await expect(page.getByText('Action needs attention · Approval pending', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Pricing requested for a 40-seat rollout', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Review action', exact: true })).toBeVisible();
  expect(mock.writes).toHaveLength(0);
});

test('evidence review is explicit and never approves the recommendation', async ({ page }) => {
  const mock = await installSignalMocks(page);
  await page.goto(`${url}&signal=signal-1&evidence_review=needs_review`);
  await page.getByText('Evidence (1)', { exact: true }).click();
  expect(mock.writes).toHaveLength(0);
  await page.getByRole('button', { name: 'Mark evidence reviewed', exact: true }).click();
  await expect(page.getByText('Evidence reviewed', { exact: true })).toBeVisible();
  expect(mock.action.status).toBe('pending');
  expect(mock.writes).toEqual([{ path: '/crm/signals/evidence-1/review', body: {} }]);
  await page.keyboard.press('Escape');
  await expect(page.getByText('No signals match this view', { exact: true })).toBeVisible();
});

test('legacy recommendation deep link resolves its linked signal without duplicate decisions', async ({ page }) => {
  const mock = await installSignalMocks(page, { standalone: true });
  mock.linked.push('signal-1'); mock.signal.actions = [mock.action]; mock.signal.pending_action_count = 1;
  await page.goto('/e2e/crm/harness/playbooks.html?review=1&recommendation=action-1');
  await expect(page.getByText('Continue in the linked signal to review this action.', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Review action', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Open linked signal', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Pricing requested for a 40-seat rollout', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Review action', exact: true })).toBeVisible();
  expect(mock.writes).toHaveLength(0);
});

test('standalone recommendations preserve exact record links and read-only permissions', async ({ page }) => {
  const mock = await installSignalMocks(page, { standalone: true, readOnly: true });
  mock.action.object_type = 'contact'; mock.action.object_id = 'contact-exact';
  await page.goto(`${url}&recommendation=action-1`);
  await expect(page.getByRole('link', { name: 'Open contact', exact: true })).toHaveAttribute('href', '/w/playbooks-test/crm/contacts/contact-exact');
  await expect(page.getByText('A teammate with CRM edit access can review this action.', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Review action', exact: true })).toHaveCount(0);
  expect(mock.writes).toHaveLength(0);
});

test('standalone stale decisions cannot retry and dismissal retains every reason', async ({ page }) => {
  const mock = await installSignalMocks(page, { standalone: true, conflict: true });
  await page.goto(`${url}&recommendation=action-1`);
  await page.getByRole('button', { name: 'Dismiss', exact: true }).click();
  await page.getByRole('combobox', { name: 'Dismissal reason' }).click();
  await expect(page.getByRole('option')).toHaveCount(7);
  await page.getByRole('option', { name: 'Wrong person or account' }).click();
  await page.getByRole('button', { name: 'Dismiss recommendation', exact: true }).click();
  await expect(page.getByText('This action has changed. Reload it before deciding.', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Dismiss recommendation', exact: true })).toBeDisabled();
  expect(mock.writes).toEqual([{ path: '/crm/signal-inbox/recommendations/action-1/dismiss', body: { revision: 'shown-revision', reason: 'wrong_entity' } }]);
});

test('historical manual approvals are not presented as completed work', async ({ page }) => {
  const mock = await installSignalMocks(page, { standalone: true });
  mock.action.status = 'accepted'; mock.action.execution_status = 'succeeded';
  await page.goto(`${url}&recommendation=action-1`);
  await expect(page.getByText('Approved — follow-through needed', { exact: true })).toBeVisible();
  await expect(page.getByText('Past decisions (1)', { exact: true })).toHaveCount(0);
  expect(mock.writes).toHaveLength(0);
});
