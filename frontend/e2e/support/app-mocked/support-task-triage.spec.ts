import { expect, test, type Page } from '@playwright/test'
import { CONVERSATION_ID, WORKSPACE_SLUG, installSupportAppMocks } from '../fixtures/supportE2E'

const assessment = {
  id: 'assessment', status: 'ready', source_kind: 'support_conversation', source_id: CONVERSATION_ID,
  teams: [{ id: 'team-1', name: 'Payments' }], labels: [],
  candidates: [{ id: 'task-1', name: 'Invoice export fails for archived invoices', display_id: 42 }],
  assessment: { actionable: true, team: { id: 'team-1', probability: .99 }, task_type: { id: 'bug', probability: .99 }, labels: [], matches: [{ task_id: 'task-1', relationship: 'duplicates', probability: .99 }], candidates_checked: 3 },
}
async function openReview(page: Page) {
  await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`)
  await page.getByRole('button', { name: /^Create task$/i }).click()
  await expect(page.getByRole('dialog', { name: /Find existing product work|Review new task/ })).toBeVisible()
}
for (const viewport of [{ width: 1440, dark: false }, { width: 390, dark: true }]) {
  test(`reviewed support matching at ${viewport.width}px`, async ({ page }, testInfo) => {
    test.setTimeout(90_000)
    await installSupportAppMocks(page, { pmAccess: true })
    await page.setViewportSize({ width: viewport.width, height: 900 })
    await page.addInitScript((dark) => { localStorage.setItem('theme', dark ? 'dark' : 'light') }, viewport.dark)
    let linked = 0
    let created = 0
    await page.route('**/api/support/inbox/conversations/*/task-triage?*', (route) => route.fulfill({ json: assessment }))
    await page.route('**/api/support/inbox/conversations/*/task-triage/review?*', async (route) => {
      expect(route.request().postDataJSON()).toEqual({ assessment_id: 'assessment', action: 'match', value: 'task-1', dismiss: false })
      linked++
      await route.fulfill({ json: { key: 'match:task-1', status: 'accepted' } })
    })
    await page.route('**/api/support/inbox/conversations/*/create-task?*', async (route) => { created++; await route.fulfill({ status: 500, json: { error: 'Should not create duplicate work' } }) })
    await openReview(page)
    await expect(page.getByText('Possible duplicate: #42 Invoice export fails for archived invoices')).toBeVisible()
    expect(linked).toBe(0)
    expect(created).toBe(0)
    const dialog = page.getByRole('dialog', { name: /Find existing product work|Review new task/ })
    await expect(dialog.getByRole('link', { name: 'this conversation’s public messages' })).toHaveAttribute('href', `/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`)
    expect(await dialog.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true)
    if (viewport.dark) await page.evaluate(() => document.documentElement.classList.add('dark'))
    await page.screenshot({ path: testInfo.outputPath(`task-matches-${viewport.width}.png`), fullPage: true })
    await dialog.getByRole('button', { name: 'Link conversation', exact: true }).focus()
    await page.keyboard.press('Enter')
    await expect(dialog).not.toBeVisible()
    expect(linked).toBe(1)
    expect(created).toBe(0)
  })
}
test('support draft stays editable after a creation error', async ({ page }, testInfo) => {
  test.setTimeout(90_000)
  await installSupportAppMocks(page, { pmAccess: true })
  await page.setViewportSize({ width: 1200, height: 900 })
  const requests: Record<string, unknown>[] = []
  await page.route('**/api/support/inbox/conversations/*/task-triage?*', (route) => route.fulfill({ json: { ...assessment, candidates: [], assessment: { ...assessment.assessment, matches: [], candidates_checked: 0 } } }))
  await page.route('**/api/support/inbox/conversations/*/task-draft?*', (route) => route.fulfill({ json: { name: 'Fix invoice export', description: '<p>CSV export fails for archived invoices.</p>', task_type: 'bug', priority: 'high', source_hash: 'public-source' } }))
  await page.route('**/api/support/inbox/conversations/*/create-task?*', async (route) => { requests.push(route.request().postDataJSON()); await route.fulfill({ status: 409, json: { error: 'The conversation changed. Refresh the draft before creating a task.' } }) })
  await openReview(page)
  await page.getByRole('button', { name: 'Draft a new task', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: /Find existing product work|Review new task/ })
  await expect(dialog.getByRole('heading', { name: 'Review new task' })).toBeVisible()
  expect(requests).toHaveLength(0)
  await dialog.getByLabel('Title', { exact: true }).fill('Fix CSV exports for archived invoices')
  await page.screenshot({ path: testInfo.outputPath('task-draft.png'), fullPage: true })
  await dialog.getByRole('button', { name: /^Create task$/i }).click()
  await expect(dialog.getByRole('alert')).toContainText('conversation changed')
  await expect(dialog.getByLabel('Title', { exact: true })).toHaveValue('Fix CSV exports for archived invoices')
  expect(requests).toHaveLength(1)
  expect(requests[0]).toMatchObject({ name: 'Fix CSV exports for archived invoices', team_id: 'team-1', task_type: 'bug', priority: 'high', reviewed_draft: true, source_hash: 'public-source' })
})
