import { expect, test } from '@playwright/test';
import { installPlaybookMocks } from '../fixtures/playbooks';
import type { CRMSuggestion } from '../../../src/lib/crmTypes';

const harness = '/e2e/crm/harness/playbooks.html?detail';

test('guided setup separates configuration, activation and existing Signal adoption', async ({ page }) => {
  const { writes, signal } = await installPlaybookMocks(page);
  signal.situation.playbook_version_id = 'version-2';
  await page.goto(harness);
  await page.getByRole('tab', { name: 'Setup', exact: true }).click();
  await page.getByRole('button', { name: 'Set up automation', exact: true }).click();
  await expect(page.getByRole('dialog')).toContainText('Connecting starts no work');
  await expect(page.getByRole('dialog')).toContainText('CRM record operations');
  expect(writes).toHaveLength(1);
  await page.getByRole('button', { name: 'Connect automation', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(writes[1].body).toMatchObject({ expected_revision: 3, expected_connection_version: 0, review_fingerprint: 'reviewed-exact-settings' });
  await page.getByRole('button', { name: 'Turn on automation', exact: true }).click();
  await expect(page.getByRole('dialog')).toContainText('No existing or paused signal starts automatically');
  expect(writes).toHaveLength(2);
  await page.getByRole('dialog').getByRole('button', { name: 'Turn on automation', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(writes[2].body).toMatchObject({ enabled: true, confirmed: true, entry_mode: 'manual', expected_revision: 0 });
  await page.screenshot({ path: '/tmp/helpin-playbook-automation-setup.png', animations: 'disabled' });
  await page.getByRole('tab', { name: 'Signals', exact: true }).click();
  await page.getByRole('button', { name: 'Harbor Labs', exact: true }).click();
  await page.getByRole('button', { name: 'Start Beacon', exact: true }).click();
  await expect(page.getByRole('dialog').last()).toContainText('No message is sent or task created without approval');
  expect(writes).toHaveLength(3);
  await page.getByRole('dialog').last().getByRole('button', { name: 'Start Beacon', exact: true }).click();
  await expect(page.getByText('Beacon is following this signal', { exact: true })).toBeVisible();
  expect(writes[3]).toMatchObject({ path: '/crm/situations/signal-1/automation', body: { expected_revision: 4, expected_generation: 0, connection_id: 'connection-1', enabled: true, confirmed: true } });
});

test('email review edits the exact proposal and never calls the ordinary composer send endpoint', async ({ page }) => {
  const { signal } = await installPlaybookMocks(page);
  const action: CRMSuggestion = { id: 'action-1', workspace_id: 'ws-playbooks', suggestion_type: 'playbook_action', title: 'Confirm the evaluation timeline', description: 'James asked for pricing.', context: { playbook_action: { version: 1, kind: 'email', title: 'Confirm the evaluation timeline', reason: 'James asked for pricing.', email: { account_id: '', contact_id: 'contact-1', to: 'james@harbor.test', subject: 'Your evaluation', body_html: '<p>Hi James, shall we discuss the timeline?</p>' } } }, status: 'pending', execution_status: 'pending', revision: 'action-revision-1', confidence: 0, created_at: new Date().toISOString(), updated_at: new Date().toISOString() };
  signal.actions = [action]; signal.pending_action_count = 1;
  const decisions: unknown[] = [];
  await page.route('**/api/crm/playbook-actions/action-1?**', (route) => route.fulfill({ json: { suggestion_id: action.id, situation_id: signal.situation.id, approver_member_id: 'member-1', expires_at: new Date(Date.now() + 3600_000).toISOString() } }));
  await page.route('**/api/crm/email/accounts?**', (route) => route.fulfill({ json: [{ id: 'mailbox-1', email_address: 'maya@helpin.test', provider: 'gmail', can_send: true, is_active: true, status: 'connected' }] }));
  await page.route('**/api/crm/situations/signal-1/actions/action-1/accept?**', async (route) => {
    decisions.push(route.request().postDataJSON()); action.status = 'accepted'; action.execution_status = 'succeeded'; signal.pending_action_count = 0;
    await route.fulfill({ json: action });
  });
  let ordinarySends = 0;
  await page.route('**/api/crm/email/send**', async (route) => { ordinarySends++; await route.abort(); });
  await page.goto(harness);
  await page.getByRole('button', { name: 'Harbor Labs', exact: true }).click();
  await page.getByRole('button', { name: 'Review action', exact: true }).click();
  await expect(page.getByRole('dialog').last()).toContainText('james@harbor.test');
  await expect(page.getByRole('button', { name: 'Send email', exact: true })).toBeDisabled();
  await page.getByRole('combobox', { name: 'Sending account' }).click();
  await page.getByRole('option', { name: 'maya@helpin.test' }).click();
  await page.getByLabel('Subject', { exact: true }).fill('Agreeing your evaluation timeline');
  await page.getByRole('textbox', { name: 'Email message' }).fill('Hi James, can we agree a timeline this week?');
  await page.screenshot({ path: '/tmp/helpin-playbook-email-review.png', animations: 'disabled' });
  expect(decisions).toHaveLength(0);
  await page.getByRole('button', { name: 'Send email', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Review action', exact: true })).toHaveCount(0);
  expect(decisions).toEqual([expect.objectContaining({ revision: 'action-revision-1', edits: { playbook_action: expect.objectContaining({ kind: 'email', email: expect.objectContaining({ account_id: 'mailbox-1', subject: 'Agreeing your evaluation timeline', body_html: expect.stringContaining('agree a timeline this week') }) }) } })]);
  expect(ordinarySends).toBe(0);
});

test('CRM readers see automation status without configuration controls', async ({ page }) => {
  await installPlaybookMocks(page, { readOnly: true });
  await page.goto(harness);
  await page.getByRole('tab', { name: 'Setup', exact: true }).click();
  await expect(page.getByText('An admin with Automation access can connect this playbook.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Set up automation', exact: true })).toHaveCount(0);
});

test('updated connections require an explicit save and existing Signals resume their pinned connection', async ({ page }) => {
  const { automation, writes, setBinding } = await installPlaybookMocks(page);
  automation.connection = { id: 'connection-new', playbook_id: 'book-1', playbook_version_id: 'version-2', version: 2, fingerprint: 'reviewed-new', execution_enabled: false, published_by_member_id: 'member-1', published_at: new Date().toISOString() };
  automation.settings = { playbook_id: 'book-1', connection_id: 'connection-old', revision: 4, enabled: true, entry_mode: 'manual', max_runs_per_day: 4, max_no_progress_runs: 3, authorized_by_member_id: 'member-1', updated_at: new Date().toISOString() };
  automation.agent_name = 'Beacon'; automation.flow_name = 'Buying-intent follow-up';
  setBinding({ situation_id: 'signal-1', playbook_id: 'book-1', connection_id: 'connection-old', enabled: false, generation: 3, no_progress_runs: 0, blocker: 'paused_by_member' });
  await page.goto(harness);
  await page.getByRole('tab', { name: 'Setup', exact: true }).click();
  await expect(page.getByText('An updated connection is ready.', { exact: false })).toBeVisible();
  expect(writes).toHaveLength(0);
  await page.getByRole('button', { name: 'Save settings', exact: true }).click();
  await expect(page.getByRole('dialog')).toContainText('Existing signals keep their pinned configuration');
  expect(writes).toHaveLength(0);
  await page.getByRole('dialog').getByRole('button', { name: 'Save settings', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(writes[0].body).toMatchObject({ connection_id: 'connection-new', expected_revision: 4, confirmed: true });
  await page.getByRole('tab', { name: 'Signals', exact: true }).click();
  await page.getByRole('button', { name: 'Harbor Labs', exact: true }).click();
  await page.getByRole('button', { name: 'Resume automation', exact: true }).click();
  await page.getByRole('dialog').last().getByRole('button', { name: 'Start Beacon', exact: true }).click();
  await expect(page.getByText('Beacon is following this signal', { exact: true })).toBeVisible();
  expect(writes[1].body).toMatchObject({ connection_id: 'connection-old', expected_generation: 3, confirmed: true });
});

test('an uncertain action is inspected without another send or approval', async ({ page }) => {
  const { signal } = await installPlaybookMocks(page);
  const action: CRMSuggestion = { id: 'action-unknown', workspace_id: 'ws-playbooks', suggestion_type: 'playbook_action', title: 'Confirm the evaluation timeline', description: 'The provider response was lost.', context: { playbook_action: { version: 1, kind: 'email', title: 'Confirm the evaluation timeline', reason: 'James requested the plan.', email: { account_id: 'mailbox-1', contact_id: 'contact-1', to: 'james@harbor.test', subject: 'Your evaluation', body_html: '<p>The agreed plan.</p>' } } }, status: 'accepted', execution_status: 'in_progress', revision: 'unknown-revision', confidence: 0, created_at: new Date().toISOString(), updated_at: new Date().toISOString() };
  signal.actions = [action]; signal.uncertain_action_count = 1;
  await page.route('**/api/crm/playbook-actions/action-unknown?**', (route) => route.fulfill({ json: { suggestion_id: action.id, situation_id: signal.situation.id, approver_member_id: 'member-1', approved_by_member_id: 'member-1', approved_at: new Date(Date.now() - 600_000).toISOString() } }));
  let checks = 0;
  await page.route('**/api/crm/playbook-actions/action-unknown/reconcile?**', (route) => { checks++; return route.fulfill({ json: action }); });
  const inspections: unknown[] = [];
  await page.route('**/api/crm/playbook-actions/action-unknown/inspect?**', (route) => { const body = route.request().postDataJSON(); inspections.push(body); action.execution_status = 'failed'; action.context.playbook_action_inspection = body; return route.fulfill({ json: action }); });
  let sends = 0;
  await page.route('**/api/crm/email/send**', async (route) => { sends++; await route.abort(); });
  await page.goto(harness);
  await page.getByRole('button', { name: 'Harbor Labs', exact: true }).click();
  await page.getByRole('button', { name: 'Check result', exact: true }).click();
  await expect(page.getByText('The result is still unconfirmed.', { exact: false })).toBeVisible();
  expect(checks).toBe(1);
  await page.getByRole('button', { name: 'Record inspected result', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Record my finding', exact: true })).toBeDisabled();
  await page.getByRole('combobox', { name: 'Inspected outcome' }).click();
  await page.getByRole('option', { name: 'The action did not complete', exact: true }).click();
  await page.getByLabel('Where did you check, and what did you find?').fill('I inspected the sending mailbox and verified that this action did not complete.');
  await page.getByRole('button', { name: 'Record my finding', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Record inspected result', exact: true })).toHaveCount(0);
  expect(inspections).toEqual([expect.objectContaining({ revision: 'unknown-revision', outcome: 'not_completed', confirmed: true })]);
  expect(sends).toBe(0);
});
