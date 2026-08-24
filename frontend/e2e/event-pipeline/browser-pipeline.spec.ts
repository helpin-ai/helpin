import { execFileSync } from 'node:child_process';
import { expect, test } from '@playwright/test';

const widgetKey = process.env.HELPIN_EVENT_TEST_WIDGET_KEY;
const projectId = process.env.EVENT_TEST_PROJECT_ID;
const frontendURL = process.env.HELPIN_EVENT_TEST_FRONTEND_URL || 'https://helpin-dev-fe.tryunhide.com';
const apiURL = process.env.HELPIN_EVENT_TEST_API_URL || 'https://helpin-dev.tryunhide.com';

test('browser events keep the authenticated project through ClickHouse', async ({ page }) => {
  test.skip(!widgetKey || !projectId, 'widget key and project ID are required');

  const events: Record<string, unknown>[] = [];
  page.on('request', (request) => {
    if (new URL(request.url()).pathname !== '/api/v1/event') return;
    const body = request.postData();
    if (!body) return;
    const parsed = JSON.parse(body);
    events.push(...(Array.isArray(parsed) ? parsed : [parsed]));
  });

  const hasEvents = (...names: string[]) =>
    names.every((name) => events.some((event) => event.event_type === name));
  const waitForEvents = (...names: string[]) =>
    expect.poll(() => hasEvents(...names), { timeout: 20_000 }).toBe(true);
  const open = async (path: string) => {
    const pageviewsBeforeNavigation = events.filter((event) => event.event_type === 'pageview').length;
    await page.goto(`${frontendURL}${path}`);
    await expect(page.locator('[data-sdk-state]')).toHaveText('SDK ready');
    await expect.poll(
      () => events.filter((event) => event.event_type === 'pageview').length,
      { timeout: 20_000 },
    ).toBeGreaterThan(pageviewsBeforeNavigation);
  };

  await open(`/event-test/?key=${encodeURIComponent(widgetKey!)}&host=${encodeURIComponent(apiURL)}&utm_source=browser-smoke`);
  const runId = (await page.locator('[data-run-id]').textContent())!.trim();
  await expect(page.locator('[data-visitor-id]')).not.toHaveText('pending');

  await page.getByRole('button', { name: 'Claim test identity' }).click();
  await waitForEvents('user_identify');
  await expect(page.locator('[data-identity-state]')).toHaveText('browser_claim / untrusted');

  await open('/event-test/pricing/');
  await page.getByRole('button', { name: 'Choose Scale' }).click();
  await waitForEvents('$interaction', 'pricing_cta_clicked');

  await open('/event-test/security/');
  await waitForEvents('article_view');
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  await expect.poll(
    () => events.some((event) => event.event_type === '$interaction' &&
      (event.event_attributes as Record<string, unknown>)?.interaction_type === 'scroll'),
    { timeout: 20_000 },
  ).toBe(true);

  await open('/event-test/demo/');
  await page.getByRole('button', { name: 'Capture demo request' }).click();
  await waitForEvents('lead', '$form', 'demo_requested');

  const serialized = JSON.stringify(events);
  expect(serialized).not.toContain('never-captured');
  expect(events.every((event) => !Object.hasOwn(event, 'project_id'))).toBe(true);
  const formEvent = events.find((event) => event.event_type === '$form')!;
  expect((formEvent.event_attributes as Record<string, unknown>).field_names).toEqual([
    'email',
    'company',
    'role',
  ]);

  let rows: Array<{
    project_id: string;
    event_types: string[];
    identity_methods: string[];
    identity_trusts: string[];
    sessions: string;
  }> = [];
  const expectedStoredEvents = ['pageview', 'pricing_cta_clicked', 'article_view', 'demo_requested'];
  await expect.poll(() => {
    const query = `SELECT toString(project_id) AS project_id, groupUniqArray(event_type) AS event_types, groupUniqArray(identity_method) AS identity_methods, groupUniqArray(identity_trust) AS identity_trusts, toString(uniqExact(session_id)) AS sessions FROM usermaven.events WHERE position(event_attributes, '${runId}') > 0 OR position(raw_event, '${runId}') > 0 GROUP BY project_id FORMAT JSONEachRow`;
    const output = execFileSync('docker', [
      'exec', 'helpin-clickhouse', 'clickhouse-client', '--user', 'helpin', '--password', 'helpin', '--query', query,
    ], { encoding: 'utf8' }).trim();
    rows = output ? output.split('\n').map((line) => JSON.parse(line)) : [];
    return rows.length === 1 && expectedStoredEvents.every((name) => rows[0].event_types.includes(name));
  }, { timeout: 60_000, intervals: [1_000, 2_000, 3_000] }).toBe(true);

  expect(rows[0].project_id.toLowerCase()).toBe(projectId!.toLowerCase());
  expect(rows[0].event_types).toEqual(expect.arrayContaining(expectedStoredEvents));
  expect(rows[0].identity_methods).toEqual(expect.arrayContaining(['anonymous', 'browser_claim']));
  expect(rows[0].identity_trusts).toEqual(['untrusted']);
  expect(Number(rows[0].sessions)).toBeGreaterThan(0);
});
