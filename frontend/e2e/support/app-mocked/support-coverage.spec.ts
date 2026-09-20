import { expect, test, type Page } from '@playwright/test'
import { installSupportAppMocks, WORKSPACE_SLUG } from '../fixtures/supportE2E'

const now = '2026-09-20T10:00:00Z'
const gap = {
  id: 'gap-1',
  workspace_id: 'ws-1',
  title: 'Customers cannot cancel a scheduled job',
  canonical_title: 'Customers cannot cancel a scheduled job',
  topic_title: '',
  gap_kind: 'action',
  v1_gap_type: 'needs_review',
  status: 'open',
  confidence: 0.9,
  evidence_count: 12,
  evidence_30d: 8,
  impact_tier: 'medium',
  first_seen_at: now,
  last_seen_at: now,
  suggestions: [],
  recommendations: [],
  evidence: [],
  related_articles: [],
}
const topic = {
  id: 'topic-1',
  title: 'Cancel a scheduled job',
  customer_need: 'Cancel a job before it runs',
  status: 'open',
  finding_count: 2,
  conversation_count: 2,
  customer_count: 2,
  updated_at: now,
}

async function setup(
  page: Page,
  options: { listFails?: boolean; detailFails?: boolean } = {},
) {
  await page.route('https://unpkg.com/react-scan/**', (route) => route.abort())
  await installSupportAppMocks(page)
  await page.route('**/api/support/coverage/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    const json = (body: unknown, status = 200) =>
      route.fulfill({
        status,
        contentType: 'application/json',
        body: JSON.stringify(body),
      })
    if (path.endsWith('/summary'))
      return json({
        total_open_gaps: 1,
        new_gaps_this_week: 1,
        gaps_fixed_this_week: 0,
        total_evidence_count: 12,
      })
    if (path.endsWith('/gaps')) {
      if (options.listFails) return json({ error: 'Unavailable' }, 503)
      const params = new URL(route.request().url()).searchParams
      return json(
        params.has('has_merge_suggestions') || params.get('status') !== 'open'
          ? { items: [], total: 0 }
          : { items: [gap], total: 1 },
      )
    }
    if (path.endsWith('/gaps/gap-1'))
      return options.detailFails
        ? json({ error: 'Unavailable' }, 503)
        : json(gap)
    if (path.endsWith('/status')) return json({ error: 'Save failed' }, 500)
    if (path.endsWith('/merge-suggestions')) return json([])
    if (path.endsWith('/latest')) return json(null)
    if (path.endsWith('/v2/health'))
      return json({
        healthy: true,
        latest_batch: null,
        failures: [],
        rollout: { read_v2_enabled: true, capture_enabled: true },
      })
    if (path.endsWith('/v2/topics')) return json({ items: [topic] })
    if (path.endsWith('/v2/topics/topic-1'))
      return json({
        topic,
        findings: [
          {
            id: 'finding-1',
            customer_need: topic.customer_need,
            ai_answer: 'Please contact support.',
            ai_failure: 'No cancellation tool was available.',
            human_answer: 'The team cancelled the job.',
            fix_type: 'add_action',
            fix_target: 'Support agent',
            rationale: 'Requires an account operation.',
            suggested_change: 'Add a guarded cancellation action.',
            conversation_id: 'conv-1',
          },
        ],
      })
    if (path.endsWith('/v2/signals')) return json({ items: [] })
    return json({ error: 'Unexpected coverage request' }, 500)
  })
  await page.route('**/api/agents?*', (route) => route.fulfill({ json: [] }))
  await page.route('**/api/docs/spaces?*', (route) =>
    route.fulfill({ json: [] }),
  )
  await page.goto(`/w/${WORKSPACE_SLUG}/support/coverage`)
  await expect(
    page.getByRole('heading', { name: 'Support coverage', exact: true }),
  ).toBeVisible({ timeout: 20_000 })
}

test('coverage separates actionable gaps from topics and links findings to source evidence', async ({
  page,
}) => {
  await setup(page)
  await expect(
    page.getByRole('heading', { name: 'Support coverage', exact: true }),
  ).toBeVisible()
  await expect(
    page.getByRole('button', { name: /Customers cannot cancel/ }),
  ).toContainText('Action or workflow')
  await expect(
    page.getByRole('button', { name: /^Cancel a scheduled job/ }),
  ).not.toBeVisible()
  await page.screenshot({ path: test.info().outputPath('coverage-desktop.png') })
  await page.getByRole('tab', { name: 'Gaps to resolve' }).focus()
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('tab', { name: 'Customer topics' })).toBeFocused()
  await expect(
    page.getByRole('button', { name: /Customers cannot cancel/ }),
  ).not.toBeVisible()
  await page.getByRole('button', { name: /^Cancel a scheduled job/ }).click()
  await expect(
    page.getByText('No cancellation tool was available.'),
  ).toBeVisible()
  await expect(page.getByText('Please contact support.')).toBeVisible()
  await expect(
    page.getByRole('link', { name: /View source conversation/ }),
  ).toHaveAttribute('href', '/w/workspace/support/conv-1')
})

test('coverage shows load failures with a retry instead of an empty success state', async ({
  page,
}) => {
  await setup(page, { listFails: true })
  await expect(page.getByRole('alert')).toContainText(
    'Could not load coverage gaps',
  )
  await expect(page.getByRole('button', { name: 'Try again' })).toBeVisible()
  await expect(page.getByText('No open coverage gaps.')).not.toBeVisible()
})

test('failed detail requests stop loading and can be retried', async ({
  page,
}) => {
  await setup(page, { detailFails: true })
  await page.getByRole('button', { name: /Customers cannot cancel/ }).click()
  await expect(page.getByRole('dialog').getByRole('alert')).toContainText(
    'Could not load this gap',
  )
  await expect(
    page.getByRole('dialog').getByRole('button', { name: 'Try again' }),
  ).toBeVisible()
})

test('failed status changes keep the gap open for review', async ({ page }) => {
  await setup(page)
  await page.getByRole('button', { name: /Customers cannot cancel/ }).click()
  await page.getByRole('button', { name: 'Mark Done' }).click()
  await expect(
    page.getByText('Could not update this gap. Your changes were not saved.'),
  ).toBeVisible()
  await expect(page.getByRole('dialog')).toBeVisible()
})

test('coverage is usable at a narrow width and in dark mode', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.emulateMedia({ colorScheme: 'dark' })
  await setup(page)
  await expect(page.locator('html')).toHaveClass(/dark/)
  await expect(
    page.getByRole('button', { name: /Customers cannot cancel/ }),
  ).toBeVisible()
  const overflowing = await page.evaluate(
    () => document.documentElement.scrollWidth > window.innerWidth,
  )
  expect(overflowing).toBe(false)
  await page.screenshot({ path: test.info().outputPath('coverage-mobile-dark.png') })
  await page.getByRole('tab', { name: 'Needs review' }).click()
  await expect(page.getByText('Nothing waiting for review')).toBeVisible()
})

test('signal review keeps evidence visible when saving fails', async ({
  page,
}) => {
  await setup(page)
  await page.route('**/api/support/coverage/v2/signals?*', (route) =>
    route.fulfill({
      json: {
        items: [
          {
            id: 'signal-1',
            normalized_query: 'Cancel the job',
            source_kind: 'widget_search',
            observed_at: now,
          },
        ],
      },
    }),
  )
  await page.route(
    '**/api/support/coverage/v2/signals/signal-1/review?*',
    (route) => route.fulfill({ status: 500, json: { error: 'Save failed' } }),
  )
  await page.getByRole('button', { name: 'Refresh', exact: true }).click()
  await page.getByRole('tab', { name: /Needs review/ }).click()
  await expect(page.getByText('Cancel the job', { exact: true })).toBeVisible()
  await expect(
    page.getByRole('button', { name: 'Attach to topic' }),
  ).toBeDisabled()
  await page.getByRole('combobox', { name: 'Topic for Cancel the job' }).click()
  await page.getByRole('option', { name: 'Cancel a scheduled job' }).click()
  await page.getByRole('button', { name: 'Attach to topic' }).click()
  await expect(
    page.getByText('Failed to attach signal', { exact: true }),
  ).toBeVisible()
  await expect(page.getByText('Cancel the job', { exact: true })).toBeVisible()
})
