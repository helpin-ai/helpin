import { test, expect } from '@playwright/test';
import { installSignalMocks } from '../fixtures/signals';

const url = '/e2e/crm/harness/playbooks.html?signals=1';

test('source drawer shows the summary once while preserving the quote and source action', async ({ page }) => {
  const mock = await installSignalMocks(page, { evidenceOnly: true });
  await page.goto(`${url}&group=${mock.evidenceGroup.id}`);
  const drawer = page.getByRole('dialog');
  await drawer.getByText('Evidence (1)', { exact: true }).click();
  await expect(drawer.getByText(mock.evidenceGroup.signals![0].summary, { exact: true })).toHaveCount(1);
  await expect(drawer.getByText(mock.evidenceGroup.signals![0].evidence_excerpt!, { exact: true })).toBeVisible();
  await expect(drawer.getByRole('link', { name: 'Open email' })).toBeVisible();
  await expect(drawer.getByRole('button', { name: 'Mark evidence reviewed' })).toBeVisible();
  await page.screenshot({ path: '/tmp/helpin-signal-drawer-source.png', fullPage: true });
  expect(mock.writes).toHaveLength(0);
});

test('multiple sources keep distinct and conflicting evidence without repeating the lead summary', async ({ page }) => {
  const mock = await installSignalMocks(page, { evidenceOnly: true });
  mock.evidenceGroup.needs_judgment = true;
  mock.evidenceGroup.signals!.push({ ...mock.evidenceGroup.signals![0], id: 'evidence-2', summary: 'The account owner says the add-on is already active.', evidence_excerpt: 'The add-on was activated yesterday.', source_type: 'support', source_thread_id: 'support-thread-1', evidence_identity_trust: 'unverified' });
  await page.goto(`${url}&group=${mock.evidenceGroup.id}`);
  const drawer = page.getByRole('dialog');
  await expect(drawer.getByText('Review the conflicting evidence before choosing a next step.')).toBeVisible();
  await drawer.getByText('Evidence (2)', { exact: true }).click();
  await expect(drawer.getByText(mock.evidenceGroup.signals![0].summary, { exact: true })).toHaveCount(1);
  await expect(drawer.getByText(mock.evidenceGroup.signals![1].summary, { exact: true })).toBeVisible();
  await expect(drawer.getByText('Customer identity is not verified for this source.')).toBeVisible();
  await expect(drawer.getByRole('link', { name: 'Open conversation' })).toHaveAttribute('href', /support-thread-1/);
  await page.screenshot({ path: '/tmp/helpin-signal-drawer-conflict.png', fullPage: true });
  expect(mock.writes).toHaveLength(0);
});

test('tracked signal does not repeat its next step as an action, objective, and evidence summary', async ({ page }) => {
  const mock = await installSignalMocks(page);
  const nextStep = mock.signal.situation.next_step;
  mock.action.title = nextStep;
  mock.action.description = nextStep;
  mock.signal.situation.objective = nextStep;
  mock.signal.evidence![0].summary = nextStep;
  await page.goto(`${url}&signal=signal-1`);
  const drawer = page.getByRole('dialog');
  await drawer.getByText('Evidence (1)', { exact: true }).click();
  await expect(drawer.getByText(nextStep, { exact: true })).toHaveCount(1);
  await expect(drawer.getByRole('button', { name: 'Edit', exact: true })).toBeVisible();
  await expect(drawer.getByRole('button', { name: 'Review action', exact: true })).toBeVisible();
  await expect(drawer.getByRole('button', { name: 'Record outcome', exact: true })).toBeVisible();
  await page.screenshot({ path: '/tmp/helpin-signal-drawer-tracked.png', fullPage: true });
  expect(mock.writes).toHaveLength(0);
});

test('standalone recommendation keeps one description and preserves read-only source access', async ({ page }) => {
  const mock = await installSignalMocks(page, { standalone: true, readOnly: true });
  mock.action.description = mock.action.title;
  mock.action.signals = [{ ...mock.signal.evidence![0], summary: mock.action.title }] as typeof mock.action.signals;
  await page.goto(`${url}&recommendation=${mock.action.id}`);
  const drawer = page.getByRole('dialog');
  await drawer.getByText('Evidence (1)', { exact: true }).click();
  await expect(drawer.getByText(mock.action.title, { exact: true })).toHaveCount(1);
  await expect(drawer.getByRole('link', { name: 'Open email' })).toBeVisible();
  await expect(drawer.getByRole('button', { name: 'Review action' })).toHaveCount(0);
  await expect(drawer.getByRole('button', { name: 'Mark evidence reviewed' })).toHaveCount(0);
  await page.screenshot({ path: '/tmp/helpin-signal-drawer-recommendation.png', fullPage: true });
  expect(mock.writes).toHaveLength(0);
});

test('long source quotes expand explicitly and keep the close control available on phones', async ({ page }) => {
  const mock = await installSignalMocks(page, { evidenceOnly: true });
  mock.evidenceGroup.signals![0].evidence_excerpt = 'Please confirm which accounts are included in our subscription. '.repeat(24) + 'The final request is a billing review.';
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${url}&group=${mock.evidenceGroup.id}&theme=dark`);
  const drawer = page.getByRole('dialog');
  await drawer.getByText('Evidence (1)', { exact: true }).click();
  const expand = drawer.getByRole('button', { name: 'Read full quote' });
  await expect(expand).toHaveAttribute('aria-expanded', 'false');
  await page.screenshot({ path: '/tmp/helpin-signal-drawer-mobile.png', fullPage: true });
  await expand.focus();
  await page.keyboard.press('Enter');
  await expect(drawer.getByRole('button', { name: 'Show less' })).toHaveAttribute('aria-expanded', 'true');
  await expect(drawer.getByText(mock.evidenceGroup.signals![0].evidence_excerpt, { exact: true })).toBeVisible();
  await drawer.getByRole('button', { name: 'Mark evidence reviewed' }).scrollIntoViewIfNeeded();
  await expect(drawer.getByRole('button', { name: 'Close', exact: true })).toBeInViewport();
  expect(await drawer.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBeTruthy();
  expect(mock.writes).toHaveLength(0);
});

test('closed signals retain their recorded outcome and source warnings', async ({ page }) => {
  const mock = await installSignalMocks(page);
  mock.signal.situation.lifecycle = 'closed';
  mock.signal.situation.outcome_kind = 'achieved';
  mock.signal.situation.outcome_summary = 'The customer confirmed the revised subscription.';
  mock.signal.evidence![0].dismissed_at = '2026-09-08T12:00:00Z';
  await page.goto(`${url}&signal=signal-1`);
  const drawer = page.getByRole('dialog');
  await expect(drawer.getByText(mock.signal.situation.outcome_summary, { exact: true })).toBeVisible();
  await drawer.getByText('Evidence (1)', { exact: true }).click();
  await expect(drawer.getByText('Dismissed evidence', { exact: false })).toBeVisible();
  await expect(drawer.getByRole('button', { name: 'Review action' })).toHaveCount(0);
  await expect(drawer.getByRole('button', { name: 'Mark evidence reviewed' })).toHaveCount(0);
  await page.screenshot({ path: '/tmp/helpin-signal-drawer-closed.png', fullPage: true });
  expect(mock.writes).toHaveLength(0);
});

test('closed signal evidence is not hidden by an old next step that is no longer displayed', async ({ page }) => {
  const mock = await installSignalMocks(page);
  mock.signal.situation.lifecycle = 'closed';
  mock.signal.situation.outcome_summary = 'The customer confirmed the new plan.';
  mock.signal.evidence![0].summary = mock.signal.situation.next_step;
  await page.goto(`${url}&signal=signal-1`);
  const drawer = page.getByRole('dialog');
  await drawer.getByText('Evidence (1)', { exact: true }).click();
  await expect(drawer.getByText(mock.signal.evidence![0].summary, { exact: true })).toBeVisible();
});

test('completed recommendation descriptions remain available as evidence', async ({ page }) => {
  const mock = await installSignalMocks(page, { standalone: true });
  mock.action.status = 'dismissed';
  mock.action.signals = [{ ...mock.signal.evidence![0], summary: mock.action.description! }] as typeof mock.action.signals;
  await page.goto(`${url}&recommendation=${mock.action.id}`);
  const drawer = page.getByRole('dialog');
  await drawer.getByText('Evidence (1)', { exact: true }).click();
  await expect(drawer.getByText(mock.action.description!, { exact: true })).toBeVisible();
});
