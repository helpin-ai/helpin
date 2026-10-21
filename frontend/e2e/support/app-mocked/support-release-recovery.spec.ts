import { expect, test, type Page } from '@playwright/test';
import { CONVERSATION_ID, installSupportAppMocks, WORKSPACE_SLUG } from '../fixtures/supportE2E';

test.setTimeout(120_000);
const sidebarChunk = /\/(?:assets\/SupportAgentSidebar-[\w-]+\.js|src\/components\/support\/SupportAgentSidebar\.tsx)(?:\?.*)?$/;
const composer = (page: Page) => page.locator('[data-support-reply-composer]:visible');

async function setup(page: Page) {
  // Deploy jobs test the real built artifact, including its production URLs.
  // Only local assets and the explicit API/upload mocks may receive requests.
  await page.route('**/*', route => new URL(route.request().url()).origin === 'http://127.0.0.1:4173'
    ? route.fallback() : route.abort());
  await installSupportAppMocks(page);
  await page.setViewportSize({ width: 1500, height: 1000 });
  await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`);
  await expect(composer(page).locator('[contenteditable=true]')).toBeVisible();
}

async function simulateRelease(page: Page, unavailable = false) {
  // The tab keeps its current entry module while the server serves a new
  // release's HTML. Missing old chunks reproduce the production SPA fallback.
  await page.route('http://127.0.0.1:4173/', route => {
    if (route.request().resourceType() === 'document') return route.fallback();
    if (unavailable) return route.abort('internetdisconnected');
    return route.fulfill({ contentType: 'text/html', body: '<div id="root"></div><script type="module" src="/assets/index-next-release.js"></script>' });
  });
  await page.route(sidebarChunk, route => route.fulfill({ contentType: 'text/html', body: '<html><div id="root"></div></html>' }));
  await page.getByRole('button', { name: 'Ask Agent about this conversation' }).click();
}

test('an open tab recovers from a removed release chunk without losing a note draft', async ({ page }) => {
  await setup(page);
  await composer(page).getByRole('button', { name: 'Note', exact: true }).click();
  await composer(page).locator('[contenteditable=true]').fill('Unsent internal note');
  let navigations = 0;
  page.on('framenavigated', frame => { if (frame === page.mainFrame()) navigations++; });
  await simulateRelease(page);
  await expect(page.getByRole('heading', { name: 'Helpin has been updated' })).toBeVisible();
  await expect(composer(page).locator('[contenteditable=true]')).toHaveText('Unsent internal note');
  expect(navigations).toBe(0);
  // Type immediately before refresh to exercise both draft-save debounce gaps.
  await composer(page).locator('[contenteditable=true]').fill('Unsent internal note, just edited');
  await page.unroute(sidebarChunk);
  const refreshed = page.waitForEvent('framenavigated', frame => frame === page.mainFrame());
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await refreshed;
  await expect(composer(page).locator('[contenteditable=true]')).toHaveText('Unsent internal note, just edited');
  await expect(composer(page).getByRole('button', { name: 'Add Note', exact: true })).toBeVisible();
  expect(navigations).toBe(1);
});

test('a connection failure offers retry and does not claim an update or refresh automatically', async ({ page }) => {
  await setup(page);
  await composer(page).locator('[contenteditable=true]').fill('Keep this reply');
  let navigations = 0;
  page.on('framenavigated', frame => { if (frame === page.mainFrame()) navigations++; });
  await simulateRelease(page, true);
  await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeEnabled();
  await expect(page.getByText('Helpin has been updated')).toHaveCount(0);
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeEnabled();
  await expect(composer(page).locator('[contenteditable=true]')).toHaveText('Keep this reply');
  expect(navigations).toBe(0);
});

test('canceling a native unsaved-changes prompt keeps recovery usable', async ({ page }) => {
  await setup(page);
  await composer(page).locator('[contenteditable=true]').fill('Keep this reply');
  await simulateRelease(page);
  await expect(page.getByRole('button', { name: 'Refresh', exact: true })).toBeEnabled();
  await page.evaluate(() => window.addEventListener('beforeunload', event => {
    event.preventDefault();
    event.returnValue = '';
  }, { once: true }));
  const canceled = page.waitForEvent('dialog').then(dialog => dialog.dismiss());
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await canceled;
  await expect(page.getByRole('button', { name: 'Refresh', exact: true })).toBeEnabled();
  await expect(composer(page).locator('[contenteditable=true]')).toHaveText('Keep this reply');
});

test('refresh waits for an upload and then preserves the uploaded attachment', async ({ page }) => {
  await setup(page);
  let completeUpload!: () => void;
  const upload = new Promise<void>(resolve => { completeUpload = resolve; });
  await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}/attachments?**`, route => route.fulfill({ json: { attachment: { id: 'draft-file' }, upload_url: 'https://example.com/recovery-upload', public_url: '' } }));
  await page.route('https://example.com/recovery-upload', async route => { await upload; await route.fulfill({ status: 200 }); });
  await page.route('**/api/support/inbox/attachments/draft-file/confirm?**', route => route.fulfill({ json: { message: 'confirmed' } }));
  await composer(page).locator('input[type=file]').setInputFiles({ name: 'draft.txt', mimeType: 'text/plain', buffer: Buffer.from('Draft attachment') });
  await simulateRelease(page);
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByText('Finish uploading or remove unfinished attachments before refreshing.')).toBeVisible();
  completeUpload();
  await expect(composer(page).getByRole('button', { name: 'Send', exact: true })).toBeEnabled();
  await page.unroute(sidebarChunk);
  const refreshed = page.waitForEvent('framenavigated', frame => frame === page.mainFrame());
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await refreshed;
  await expect(composer(page)).toContainText('draft.txt');
  await expect(composer(page).getByRole('button', { name: 'Send', exact: true })).toBeEnabled();
});
