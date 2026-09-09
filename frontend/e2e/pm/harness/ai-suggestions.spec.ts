import { test, expect, type Page } from '@playwright/test';

const suggestion = { id: 'suggestion-1', title: 'Confirm the release checklist', meeting_id: 'meeting-1', meeting_title: 'Engineering weekly sync', meeting_at: '2026-09-08T10:00:00Z', created_at: '2026-09-09T09:00:00Z' };
const draft = { ...suggestion, draft_subject: suggestion.title, draft_body: 'Please confirm the release checklist and share the remaining blockers before Thursday.', revision: 'revision-1' };
async function setup(page: Page, options: { readOnly?: boolean; crm?: boolean; empty?: boolean; failList?: boolean; stale?: boolean; long?: boolean; pages?: boolean; billing?: boolean } = {}) {
  let reviewed = false;
  let failList = !!options.failList;
  const writes: { path: string; body: unknown }[] = [];
  await page.route('**/*', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (!url.pathname.startsWith('/api/') && !url.pathname.endsWith('/v1/event')) return route.continue();
    const path = url.pathname.replace(/^\/api/, '');
    const headers = { 'access-control-allow-origin': request.headers().origin || '*', 'access-control-allow-credentials': 'true' };
    const json = (body: unknown, status = 200) => route.fulfill({ status, headers, contentType: 'application/json', body: JSON.stringify(body) });
    if (request.method() === 'OPTIONS') return route.fulfill({ status: 204, headers: { ...headers, 'access-control-allow-headers': 'authorization, content-type', 'access-control-allow-methods': 'GET, POST, OPTIONS' } });
    if (path.endsWith('/v1/event')) return json({});
    if (path.endsWith('/me')) return json({ membership: { id: 'member-1', role: 'admin' }, permissions: ['pm.read', ...(!options.readOnly ? ['pm.edit'] : []), ...(options.crm ? ['crm.read', 'crm.edit'] : [])], modules: ['pm'], team_memberships: [] });
    if (path.endsWith('/teams')) return json([]);
    if (path === '/pm/tasks') return json({ data: [], total: 0 });
    if (request.method() === 'POST') {
      writes.push({ path, body: request.postDataJSON() });
      if (path.endsWith('/recheck-routing')) {
        if (options.billing) return json({ items: [{ id: 'first', title: 'Completed recap', outcome: 'internal' }], billing_error: 'AI completion failed: AI allowance exhausted' });
        return json(url.searchParams.has('cursor') ? { items: [] } : { items: [{ id: 'one', title: 'Product team recap', outcome: 'internal' }, { id: 'two', title: 'Customer proposal', outcome: 'customer' }, { id: 'three', title: 'Recording without transcript', outcome: 'missing_transcript' }], next_cursor: 'cursor-1' });
      }
      if (options.stale) return json({ error: 'stale' }, 409);
      reviewed = true;
      return json({ id: suggestion.id, status: path.endsWith('/accept') ? 'accepted' : 'dismissed', execution_status: 'manual_required' });
    }
    if (path === '/pm/ai-suggestions') {
      if (failList) { failList = false; return json({ error: 'unavailable' }, 503); }
      const pageNumber = Number(url.searchParams.get('page') || 1);
      return json({ data: reviewed || options.empty ? [] : [{ ...suggestion, id: pageNumber > 1 ? 'suggestion-2' : suggestion.id }], total: reviewed || options.empty ? 0 : options.pages ? 26 : 1, page: pageNumber, per_page: 25, total_pages: options.pages ? 2 : 1 });
    }
    if (path.startsWith('/pm/ai-suggestions/')) return json({ ...draft, ...(options.long ? { draft_body: Array.from({ length: 50 }, (_, i) => `Follow-up paragraph ${i + 1}: confirm a specific next step with the engineering team.`).join('\n\n') } : {}) });
    return json({ error: `Unexpected fixture request: ${path}` }, 404);
  });
  await page.goto('/e2e/pm/harness/my-work.html');
  await page.getByRole('tab', { name: 'AI suggestions', exact: true }).click();
  await expect(page.getByRole('tabpanel', { name: 'AI suggestions', exact: true })).toBeVisible();
  return writes;
}

test('shows a concise source-backed list and records review without sending or creating tasks', async ({ page }) => {
  const writes = await setup(page, { crm: true });
  await expect(page.getByText(suggestion.meeting_title, { exact: false })).toBeVisible();
  await expect(page.getByText(draft.draft_body)).toHaveCount(0);
  await page.screenshot({ path: '/tmp/helpin-my-work-ai-list.png' });
  await page.getByRole('button', { name: /Confirm the release checklist/ }).click();
  const drawer = page.getByRole('dialog');
  await expect(drawer.getByText(draft.draft_body)).toBeVisible();
  await expect(drawer.getByText(suggestion.title, { exact: true })).toHaveCount(1);
  await expect(drawer.getByRole('link', { name: suggestion.meeting_title })).toHaveAttribute('href', '/w/acme/crm/meetings/meeting-1');
  await expect(drawer.getByText('Marking reviewed records your review. Nothing is sent.')).toBeVisible();
  await page.screenshot({ path: '/tmp/helpin-my-work-ai-drawer.png' });
  await drawer.getByRole('button', { name: 'Mark reviewed', exact: true }).click();
  await expect(drawer).toHaveCount(0);
  await expect(page.getByText("You're all caught up")).toBeVisible();
  await expect(page.getByRole('heading', { name: 'AI suggestions', exact: true })).toBeFocused();
  expect(writes).toEqual([{ path: '/pm/ai-suggestions/suggestion-1/accept', body: { revision: 'revision-1' } }]);
});

test('dismisses the same suggestion and rejects stale review without losing its draft', async ({ page }) => {
  const writes = await setup(page, { stale: true });
  await page.getByRole('button', { name: /Confirm the release checklist/ }).click();
  await page.getByRole('button', { name: 'Dismiss', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('changed or was already reviewed');
  await expect(page.getByText(draft.draft_body)).toBeVisible();
  expect(writes).toEqual([{ path: '/pm/ai-suggestions/suggestion-1/dismiss', body: { revision: 'revision-1' } }]);
});

test('dismisses a suggestion without creating other work', async ({ page }) => {
  const writes = await setup(page);
  await page.getByRole('button', { name: /Confirm the release checklist/ }).click();
  await page.getByRole('button', { name: 'Dismiss', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByText("You're all caught up")).toBeVisible();
  expect(writes).toEqual([{ path: '/pm/ai-suggestions/suggestion-1/dismiss', body: { revision: 'revision-1' } }]);
});

test('PM-only read access preserves provenance without CRM links or mutation controls', async ({ page }) => {
  const writes = await setup(page, { readOnly: true });
  await page.getByRole('button', { name: /Confirm the release checklist/ }).click();
  await expect(page.getByRole('dialog').getByText(suggestion.meeting_title, { exact: true })).toBeVisible();
  await expect(page.getByRole('dialog').getByRole('link')).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Mark reviewed', exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Dismiss', exact: true })).toHaveCount(0);
  expect(writes).toEqual([]);
});

test('supports list retry and pagination', async ({ page }) => {
  await setup(page, { failList: true, pages: true });
  await expect(page.getByText('Unable to load suggestions')).toBeVisible();
  await page.getByRole('button', { name: 'Try again', exact: true }).click();
  await expect(page.getByText('1 of 2')).toBeVisible();
  await page.getByRole('button', { name: 'Next', exact: true }).click();
  await expect(page.getByText('2 of 2')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Next', exact: true })).toBeDisabled();
});

test('keeps long drafts and footer usable on a narrow dark screen and restores keyboard focus', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await setup(page, { long: true });
  await page.evaluate(() => document.documentElement.classList.add('dark'));
  const row = page.getByRole('button', { name: /Confirm the release checklist/ });
  await row.focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('button', { name: 'Mark reviewed', exact: true })).toBeInViewport();
  await expect(page.getByRole('button', { name: 'Close', exact: true })).toBeInViewport();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: '/tmp/helpin-my-work-ai-mobile.png' });
  await page.keyboard.press('Escape');
  await expect(row).toBeFocused();
});


test('rechecks routing with concise outcomes and checks the next batch without reviewing drafts', async ({ page }) => {
  const writes = await setup(page, { crm: true, empty: true });
  await page.getByRole('button', { name: 'Recheck routing', exact: true }).click();
  await expect(page.getByText('Product team recap', { exact: true })).toBeVisible();
  await expect(page.getByText('Moved to the recipient’s AI suggestions', { exact: true })).toBeVisible();
  await expect(page.getByText('Customer-related · stays in CRM', { exact: true })).toBeVisible();
  await expect(page.getByText('Transcript unavailable · stays in CRM', { exact: true })).toBeVisible();
  await page.screenshot({ path: '/tmp/helpin-ai-routing-recheck.png' });
  await page.getByRole('button', { name: 'Check more', exact: true }).click();
  await expect(page.getByText('No more unchecked meeting follow-ups found.', { exact: true })).toBeVisible();
  expect(writes.every(write => write.path === '/pm/ai-suggestions/recheck-routing')).toBe(true);
  expect(writes).toHaveLength(2);
});

test('routing recheck is unavailable without CRM access and reports AI billing failures', async ({ page }) => {
  await setup(page, { readOnly: true });
  await expect(page.getByRole('button', { name: 'Recheck routing', exact: true })).toHaveCount(0);
  await page.unrouteAll({ behavior: 'wait' });
  await setup(page, { crm: true, billing: true });
  await page.getByRole('button', { name: 'Recheck routing', exact: true }).click();
  await expect(page.getByRole('dialog')).toBeVisible();
  await expect(page.getByRole('dialog')).toContainText(/AI usage|AI credits|allowance/i);
  await page.keyboard.press('Escape');
  await expect(page.getByText('Completed recap', { exact: true })).toBeVisible();
});
