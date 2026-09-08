import { chromium, expect } from '@playwright/test';

const base = process.env.CRM_PREVIEW_URL || 'http://127.0.0.1:5187/design-preview/signals/';
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
page.setDefaultTimeout(8000);
const errors = [];
page.on('pageerror', error => errors.push(error.message));
const drawer = page.locator('.work-preview-sheet');
const modal = page.locator('.work-modal');
const open = async (query = '') => {
  await page.goto(`${base}${query ? `?${query}` : ''}`, { waitUntil: 'networkidle' });
  await expect(page.locator('.work-preview')).toBeVisible();
};
const pick = async (label, value) => {
  await page.getByRole('combobox', { name: label, exact: true }).click();
  await page.getByRole('option', { name: value, exact: true }).click();
};
const screenshot = async name => {
  await page.screenshot({ path: `/tmp/crm-design-${name}.png`, animations: 'disabled' });
};
const check = async (name, run) => {
  await run();
  console.log(`PASS ${name}`);
};

try {
  await check('Queue: all five columns, main filters, category counts, search', async () => {
    await open();
    await expect(page.getByTestId('situation-row')).toHaveCount(7);
    await expect(page.locator('.signals-grid th')).toHaveText(['Signal', 'Customer', 'Category', 'Owner', 'Priority']);
    for (const label of ['Assignment', 'Work state', 'Evidence review', 'Priority']) await expect(page.getByRole('combobox', { name: label, exact: true })).toBeVisible();
    await screenshot('01-signals');
    await page.getByRole('tab', { name: /^Retention/ }).click();
    await expect(page.getByTestId('situation-row')).toHaveCount(1);
    await expect(page.locator('.signals-grid th')).toHaveCount(5);
    await page.getByRole('tab', { name: /^All / }).click();
    await page.getByLabel('Search signals', { exact: true }).fill('Harbor');
    await expect(page.getByTestId('situation-row')).toHaveCount(1);
  });
  await check('Buying intent: edit exact response, approve once, wait for customer', async () => {
    await open('situation=harbor-buyer');
    await expect(drawer.getByRole('button', { name: 'Approve & send', exact: true })).toBeVisible();
    await screenshot('02-buyer');
    await drawer.locator('[contenteditable=true]').fill('Hi James, would Thursday at 3 PM work for your team?');
    await drawer.getByRole('button', { name: 'Approve & send', exact: true }).click();
    await expect(drawer.getByText('Waiting', { exact: true })).toBeVisible();
    await expect(drawer.getByRole('button', { name: 'Approve & send', exact: true })).toHaveCount(0);
    await drawer.getByRole('button', { name: 'Close situation' }).click();
    await expect(page.getByTestId('situation-row')).toHaveCount(6);
    await pick('Work state', 'Waiting');
    await expect(page.getByText('Pricing requested for a 40-seat rollout', { exact: true })).toBeVisible();
  });
  await check('Handoff: preserve commitments, transfer ownership, no automatic task', async () => {
    await open('situation=lumen-handoff');
    await screenshot('03-handoff');
    await drawer.getByRole('button', { name: 'Accept handoff', exact: true }).click();
    await expect(drawer.getByText(/Onboarding is still in progress/)).toBeVisible();
    await expect(drawer.getByRole('combobox', { name: 'Situation owner' })).toContainText('Muhammad Azhar');
    await expect(drawer.getByText('Existing work', { exact: true })).toHaveCount(0);
  });
  await check('Failure: known non-delivery, reconnect confirmation, single approved send', async () => {
    await open('situation=orbit-delivery');
    await expect(drawer.getByText('Nothing was sent', { exact: true })).toBeVisible();
    await screenshot('04-delivery-failure');
    await drawer.getByRole('button', { name: 'Reconnect & send', exact: true }).click();
    await expect(modal.getByRole('heading', { name: 'Reconnect and send' })).toBeVisible();
    await modal.getByRole('button', { name: 'Simulate reconnect & send' }).click();
    await expect(drawer.getByText('Waiting', { exact: true })).toBeVisible();
  });
  await check('Renewal: existing work, pause and resume without resolving the situation', async () => {
    await open('situation=northstar-risk');
    await screenshot('05-renewal');
    await drawer.getByRole('button', { name: /ENG-248/ }).click();
    await expect(modal).toContainText('Omar Farooq');
    await modal.getByRole('button', { name: 'Back to work' }).click();
    await drawer.getByRole('button', { name: 'More situation actions' }).click();
    await page.getByRole('menuitem', { name: 'Pause customer work', exact: true }).click();
    await modal.getByRole('textbox', { name: 'Decision reason' }).fill('Waiting for a confirmed engineering timeline.');
    await modal.getByRole('button', { name: 'Pause work', exact: true }).click();
    await expect(drawer.getByText('Work is paused', { exact: true })).toBeVisible();
    await drawer.getByRole('button', { name: 'Resume work', exact: true }).click();
    await expect(drawer.getByText('Needs approval', { exact: true })).toBeVisible();
  });
  await check('Renewal outcomes: blocker confirmation does not silently complete the renewal', async () => {
    await open('situation=northstar-risk');
    await drawer.getByRole('button', { name: 'More situation actions' }).click();
    await page.getByRole('menuitem', { name: 'Record an outcome', exact: true }).click();
    await pick('Blocker outcome', 'Customer confirmed resolution');
    await modal.getByRole('textbox', { name: 'Confirmed outcome' }).fill('Anna confirmed the sync fix in today’s support conversation. Procurement is still reviewing the renewal.');
    await screenshot('27-renewal-milestones');
    await modal.getByRole('button', { name: 'Record outcome', exact: true }).click();
    await expect(drawer.getByText('Waiting', { exact: true })).toBeVisible();
    await expect(drawer.getByText(/Renewal confirmation needed/)).toBeVisible();
    await drawer.getByRole('button', { name: 'Record outcome', exact: true }).click();
    await pick('Blocker outcome', 'Customer confirmed resolution');
    await pick('Renewal outcome', 'Customer confirmed renewal');
    await modal.getByRole('textbox', { name: 'Confirmed outcome' }).fill('Anna signed the annual renewal and confirmed by email.');
    await modal.getByRole('button', { name: 'Record outcome', exact: true }).click();
    await expect(drawer.getByText('Resolved', { exact: true })).toBeVisible();
    await expect(drawer.getByText(/Recorded from your decision/)).toBeVisible();
    await screenshot('28-confirmed-renewal');
  });
  await check('Standalone review: edit exact CRM fields, apply without claiming customer success', async () => {
    await open('situation=cedar-deal');
    await expect(drawer.getByRole('table', { name: 'Proposed CRM changes' })).toBeVisible();
    await drawer.getByRole('textbox', { name: 'Proposed Deal name' }).fill('Cedar · Operations platform');
    await screenshot('06-crm-change');
    await drawer.getByRole('button', { name: 'Approve & apply' }).click();
    await expect(drawer.getByText('CRM change applied', { exact: true })).toBeVisible();
    await expect(drawer.getByText('Waiting', { exact: true })).toBeVisible();
    await expect(drawer.getByText('Cedar · Operations platform', { exact: true })).toBeVisible();
  });
  await check('Human feedback: reject proposal with a reason and preserve the situation', async () => {
    await open('situation=harbor-buyer');
    await drawer.getByRole('button', { name: 'Don’t send', exact: true }).click();
    await modal.getByRole('textbox', { name: 'Decision reason' }).fill('Ask about their timezone before proposing a time.');
    await modal.getByRole('button', { name: 'Request changes', exact: true }).click();
    await expect(drawer.getByText('Needs attention', { exact: true })).toBeVisible();
    await expect(drawer.getByRole('button', { name: 'Approve revised response' })).toBeVisible();
  });
  await check('Optional task: explicitly create, keep customer outcome open', async () => {
    await open('scope=Unassigned&state=All+open&situation=beacon-owner');
    await drawer.getByRole('button', { name: 'Assign to me', exact: true }).click();
    await drawer.getByRole('button', { name: 'More situation actions' }).click();
    await page.getByRole('menuitem', { name: 'Create a follow-up task', exact: true }).click();
    await modal.getByRole('textbox', { name: 'Task name' }).fill('Confirm access requirements with Nina');
    await pick('Task owner', 'Daniel Kim');
    await screenshot('07-optional-task');
    await modal.getByRole('button', { name: 'Create & link task' }).click();
    await expect(drawer.getByRole('button', { name: /Confirm access requirements/ })).toContainText('Daniel Kim');
    await expect(drawer.getByText('Needs attention', { exact: true })).toBeVisible();
  });
  await check('Playbooks: catalogue, overview, customer progress, outcomes and enrollment controls', async () => {
    await open('view=playbooks');
    await expect(page.getByRole('table', { name: 'Playbooks', exact: true }).locator('tbody tr')).toHaveCount(3);
    await screenshot('08-playbooks');
    await page.getByRole('button', { name: 'Renewal-risk recovery', exact: true }).click();
    await screenshot('09-playbook-overview');
    await page.getByRole('button', { name: 'Disable new enrollments' }).click();
    await expect(page.getByRole('button', { name: 'Apply to customer' })).toBeDisabled();
    await expect(page.getByRole('button', { name: /1 active customers/ })).toBeVisible();
    await page.getByRole('button', { name: 'Enable new enrollments' }).click();
    await page.getByRole('tab', { name: /^Customers/ }).click();
    await expect(page.getByRole('table', { name: 'Participating customers' }).locator('tbody tr')).toHaveCount(2);
    await screenshot('10-participating-customers');
    await page.getByRole('tab', { name: /^Outcomes/ }).click();
    await expect(page.getByRole('table', { name: 'Playbook outcomes' })).toContainText('Ember');
    await screenshot('11-outcomes');
  });
  await check('Configuration: all sections, draft, preview and publish; active process remains pinned', async () => {
    await open('view=edit&id=renewal-recovery');
    await screenshot('12-editor-basics');
    await page.getByRole('textbox', { name: 'Playbook name', exact: true }).fill('Renewal recovery · Enterprise');
    await page.getByRole('button', { name: '2 Entry & ownership' }).click();
    await screenshot('13-entry-ownership');
    await page.getByRole('button', { name: '3 Steps', exact: true }).click();
    await page.getByRole('textbox', { name: 'Step 1 name' }).fill('Review the enterprise renewal');
    await screenshot('14-editor-steps');
    await page.getByRole('button', { name: '4 Permissions', exact: true }).click();
    await screenshot('15-permissions');
    await page.getByRole('button', { name: '5 Stop conditions', exact: true }).click();
    await screenshot('16-stop-conditions');
    await page.getByRole('button', { name: 'Preview & publish', exact: true }).click();
    await expect(modal.getByRole('button', { name: 'Publish version 5' })).toBeDisabled();
    await modal.getByRole('button', { name: 'Run sample check', exact: true }).click();
    await screenshot('17-publish-preview');
    await modal.getByRole('button', { name: 'Publish version 5' }).click();
    await expect(page.getByRole('heading', { name: 'Renewal recovery · Enterprise', exact: true })).toBeVisible();
    await page.getByRole('tab', { name: /^Customers/ }).click();
    await page.getByRole('button', { name: 'Northstar', exact: true }).click();
    await drawer.getByRole('button', { name: 'View customer progress' }).click();
    await expect(page.getByText('Understand the renewal risk', { exact: true })).toBeVisible();
    await expect(page.getByText('Review the enterprise renewal', { exact: true })).toHaveCount(0);
  });
  await check('Create a complete playbook from a configurable template', async () => {
    await open('view=playbooks');
    await page.getByRole('button', { name: 'Create playbook', exact: true }).click();
    await screenshot('29-create-playbook');
    await modal.getByRole('button', { name: /^Buying-intent follow-up/ }).click();
    await page.getByRole('textbox', { name: 'Playbook name', exact: true }).fill('Enterprise buying process');
    await page.getByRole('button', { name: 'Preview & publish', exact: true }).click();
    await modal.getByRole('button', { name: 'Run sample check', exact: true }).click();
    await modal.getByRole('button', { name: 'Publish version 1' }).click();
    await expect(page.getByRole('heading', { name: 'Enterprise buying process', exact: true })).toBeVisible();
    await expect(page.getByText('Published version 1', { exact: true })).toBeVisible();
  });
  await check('Manual enrollment: single customer process, no automatic message or task', async () => {
    await open('view=playbook&id=buying-intent');
    await page.getByRole('button', { name: 'Apply to customer' }).click();
    await screenshot('18-manual-enrollment');
    await modal.getByRole('button', { name: 'Apply playbook' }).click();
    await expect(drawer.getByRole('button', { name: 'Aster', exact: true })).toBeVisible();
    await expect(drawer.getByText('Needs approval', { exact: true })).toBeVisible();
    await expect(drawer.getByText('Existing work', { exact: true })).toHaveCount(0);
  });
  await check('Customer continuity and shared Flow, Agent, Activity destinations', async () => {
    await open('view=customer&id=Northstar');
    await expect(page.locator('.customer-situation')).toHaveCount(2);
    await screenshot('19-customer-record');
    await page.getByRole('tab', { name: /^Linked work/ }).click();
    await expect(page.getByRole('button', { name: /ENG-248/ })).toBeVisible();
    await open('view=flows&id=renewal-recovery');
    await screenshot('20-connected-flow');
    await page.getByRole('button', { name: 'CRM Assistant', exact: true }).click();
    await expect(page.getByRole('table', { name: 'Agent tool permissions' })).toBeVisible();
    await screenshot('21-shared-agent');
    await page.getByRole('navigation', { name: 'Automation', exact: true }).getByRole('button', { name: 'Activity', exact: true }).click();
    await expect(page.getByRole('table', { name: 'Automation activity' })).toBeVisible();
    await screenshot('22-execution-activity');
  });
  await check('Preview guide and non-happy-path states', async () => {
    await open();
    await page.getByRole('button', { name: 'Explore screens', exact: true }).click();
    await screenshot('23-screen-guide');
    await modal.getByRole('button', { name: /^Signals · My next actions/ }).click();
    await pick('Preview state', 'Empty');
    await expect(page.getByRole('button', { name: /Show sample data/ })).toBeVisible();
    await screenshot('24-empty');
    await pick('Preview state', 'Error');
    await expect(page.getByRole('button', { name: 'Try again', exact: true })).toBeVisible();
    await screenshot('25-error');
    await page.getByRole('button', { name: 'Try again', exact: true }).click();
    await expect(page.getByTestId('situation-row')).toHaveCount(7);
    await pick('Preview state', 'Read-only');
    await page.getByRole('button', { name: 'Pricing requested for a 40-seat rollout', exact: true }).click();
    await expect(drawer.getByRole('combobox', { name: 'Situation owner' })).toBeDisabled();
    await expect(drawer.getByRole('button', { name: 'Approve & send' })).toHaveCount(0);
    await screenshot('26-read-only');
    await drawer.getByRole('button', { name: 'Close situation' }).click();
    await page.getByRole('button', { name: 'Reset', exact: true }).click();
    await expect(page.getByTestId('situation-row')).toHaveCount(7);
    await expect(page.locator('body')).not.toHaveCSS('overflow-x', 'scroll');
    const hasOverflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
    expect(hasOverflow).toBe(false);
  });
  expect(errors).toEqual([]);
  console.log('PASS No browser runtime errors. Desktop screenshots saved under /tmp/crm-design-*.png.');
} catch (error) {
  await page.screenshot({ path: '/tmp/crm-design-test-failure.png' });
  console.error(error);
  process.exitCode = 1;
} finally {
  await browser.close();
}
