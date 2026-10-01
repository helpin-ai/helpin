import { expect, test, type Page } from '@playwright/test';
import { installSupportAppMocks, WORKSPACE_ID } from '../fixtures/supportE2E';

async function installKnowledgeMocks(page: Page, failOnce = false, indexing = { complete: false }) {
  await installSupportAppMocks(page, { pmAccess: true });
  const requests: string[] = [];
  let failed = false;
  await page.route('**/api/**', async route => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace('/api/pm/', '/api/automation/');
    const json = (body: unknown, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
    if (path === `/api/workspaces/${WORKSPACE_ID}/me`) return json({
      workspace_id: WORKSPACE_ID, modules: ['support', 'pm', 'docs'],
      membership: { role: 'owner', status: 'active' }, team_memberships: [],
      permissions: ['workspace.read', 'settings.read', 'settings.manage', 'support.read', 'support.edit', 'support.admin', 'pm.read', 'pm.edit', 'docs.read', 'docs.edit'],
    });
    if (path === '/api/support/inbox/installations') return json({ settings: { ai_agent_id: 'support-agent' } });
    if (path === '/api/docs/spaces') return json([{ id: 'space', workspace_id: WORKSPACE_ID, name: 'Help center', type: 'external_capable', slug: 'help-center' }]);
    if (path === '/api/docs/documents' || path === '/api/docs/collections') return json([]);
    if (path === '/api/automation/content-sources') return json([
      { id: 'website', workspace_id: WORKSPACE_ID, name: 'Product website', start_url: 'https://example.com', sync_status: 'ready', indexed_pages: 2, indexed_chunks: 4 },
      { id: 'file', workspace_id: WORKSPACE_ID, name: 'Support handbook', source_type: 'file', file_name: 'handbook.pdf', sync_status: 'ready', indexed_pages: 1, indexed_chunks: 2 },
      { id: 'empty', workspace_id: WORKSPACE_ID, name: 'New website', start_url: 'https://new.example.com', sync_status: indexing.complete ? 'ready' : 'queued', indexed_pages: indexing.complete ? 1 : 0 },
    ]);
    if (path === '/api/automation/agents/support-agent/knowledge-sources') return json([
      { id: 'collection-source', agent_id: 'support-agent', workspace_id: WORKSPACE_ID, space_id: 'space', scope_type: 'collection', collection_id: 'billing', collection_name: 'Billing guidance', sync_status: 'ready', indexed_documents: 1 },
      { id: 'article-source', agent_id: 'support-agent', workspace_id: WORKSPACE_ID, space_id: 'space', scope_type: 'article', document_id: 'invoice', document_title: 'Invoice corrections', sync_status: 'ready', indexed_documents: 1 },
    ]);
    if (path.endsWith('/indexed-documents')) {
      requests.push(path);
      return json([{ id: 'invoice', title: 'Invoice corrections' }]);
    }
    if (/\/content-sources\/[^/]+\/pages$/.test(path)) {
      requests.push(url.pathname + url.search);
      expect(url.searchParams.get('indexed')).toBe('true');
      if (path.includes('/empty/')) return json(indexing.complete ? [{ id: 'new-page', title: 'Getting started', url: 'https://new.example.com/start', http_status: 200 }] : []);
      if (path.includes('/file/')) return json([{ id: 'file-page', title: 'Handbook — page 1', url: 'file://handbook.pdf', http_status: 200 }]);
      if (failOnce && !failed) { failed = true; return json({ error: 'temporary failure' }, 500); }
      return json([
        { id: 'pricing', title: 'Plans and pricing', url: 'https://example.com/pricing', http_status: 200 },
        { id: 'billing', title: 'Billing FAQ', url: 'https://example.com/billing', http_status: 200 },
      ]);
    }
    if (path.endsWith('/pages/file-page')) return json({ id: 'file-page', title: 'Handbook — page 1', url: 'file://handbook.pdf', content_text: 'Billing reviews invoice correction requests.' });
    return route.fallback();
  });
  return requests;
}

async function openKnowledge(page: Page) {
  await page.goto('/w/workspace/settings/knowledge', { waitUntil: 'domcontentloaded' });
  await expect(page.getByText('Product website', { exact: true })).toBeVisible({ timeout: 90_000 });
  await page.addStyleTag({ content: '.helpin-query-devtools { display: none !important; }' });
}

test.beforeEach(async ({ page }) => {
  test.setTimeout(150_000);
  await page.setViewportSize({ width: 1440, height: 1000 });
});

test('indexed counts open searchable source drawers through keyboard-accessible links', async ({ page }, testInfo) => {
  const requests = await installKnowledgeMocks(page);
  await openKnowledge(page);
  expect(requests).toEqual([]);
  const row = page.getByRole('row').filter({ hasText: 'Product website' });
  const count = row.getByRole('button', { name: 'View indexed content in Product website' });
  await count.focus({ timeout: 10_000 });
  await expect(page.getByRole('tooltip')).toHaveText('View indexed content');
  await page.screenshot({ path: testInfo.outputPath('knowledge-indexed-count-tooltip.png'), animations: 'disabled' });
  await count.press('Enter');
  const drawer = page.getByRole('dialog', { name: 'Indexed content' });
  await expect(drawer.getByText('Plans and pricing', { exact: true })).toBeVisible();
  await expect(drawer.getByRole('link', { name: 'Open Plans and pricing' })).toHaveAttribute('href', 'https://example.com/pricing');
  await drawer.getByRole('searchbox', { name: 'Search indexed content' }).fill('billing');
  await expect(drawer.getByText('Plans and pricing', { exact: true })).toHaveCount(0);
  await expect(drawer.getByText('Billing FAQ', { exact: true })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('knowledge-indexed-desktop.png'), animations: 'disabled' });
  await page.keyboard.press('Escape');
  await expect(count).toBeFocused();
  await page.getByRole('row').filter({ hasText: 'Billing guidance' }).getByRole('button', { name: 'View indexed content in Billing guidance' }).click();
  await expect(drawer.getByRole('link', { name: 'Invoice corrections' })).toHaveAttribute('href', '/w/workspace/docs/documents/invoice');
  expect(requests.at(-1)).toContain('/collection-source/indexed-documents');
  await page.keyboard.press('Escape');
  await page.getByRole('row').filter({ hasText: 'Invoice corrections' }).getByRole('button', { name: 'View indexed content in Invoice corrections' }).click();
  await expect(drawer.getByRole('link', { name: 'Invoice corrections' })).toBeVisible();
  expect(requests.at(-1)).toContain('/article-source/indexed-documents');
});

test('indexed content supports file preview, retry and empty mobile dark states', async ({ page }, testInfo) => {
  await installKnowledgeMocks(page, true);
  await openKnowledge(page);
  await page.getByRole('button', { name: 'View indexed content in Product website' }).click();
  const drawer = page.getByRole('dialog', { name: 'Indexed content' });
  await expect(drawer.getByText('Could not load indexed content.')).toBeVisible();
  await drawer.getByRole('button', { name: 'Retry' }).click();
  await expect(drawer.getByText('Plans and pricing', { exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: 'View indexed content in Support handbook' }).click();
  await drawer.getByRole('button', { name: 'Handbook — page 1', exact: true }).click();
  await expect(drawer.getByText('Billing reviews invoice correction requests.')).toBeVisible();
  await expect(drawer.locator('a[href^="file:"]')).toHaveCount(0);
  await page.keyboard.press('Escape');
  await page.setViewportSize({ width: 390, height: 844 });
  await page.evaluate(() => document.documentElement.classList.add('dark'));
  await page.getByRole('button', { name: 'View indexed content in Product website' }).click();
  await expect(drawer.getByText('Plans and pricing', { exact: true })).toBeVisible();
  expect(await drawer.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath('knowledge-indexed-mobile-populated.png'), animations: 'disabled' });
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: 'View indexed content in New website' }).click();
  await expect(drawer.getByText('No indexed content yet.')).toBeVisible();
  await expect(drawer.getByText('Indexing is in progress. Content will appear here when it’s ready.')).toBeVisible();
  expect(await drawer.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath('knowledge-indexed-mobile-dark.png'), animations: 'disabled' });
});

test('an open indexed drawer refreshes when indexing finishes', async ({ page }) => {
  const indexing = { complete: false };
  await installKnowledgeMocks(page, false, indexing);
  await openKnowledge(page);
  await page.getByRole('button', { name: 'View indexed content in New website' }).click();
  const drawer = page.getByRole('dialog', { name: 'Indexed content' });
  await expect(drawer.getByText('No indexed content yet.')).toBeVisible();
  indexing.complete = true;
  await expect(drawer.getByText('Getting started', { exact: true })).toBeVisible({ timeout: 15_000 });
  await expect(drawer.getByText('· 1 page', { exact: true })).toBeVisible();
});
