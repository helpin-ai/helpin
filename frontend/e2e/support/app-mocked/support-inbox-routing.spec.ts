import { expect, test } from '@playwright/test'
import { buildConversation, CONVERSATION_ID, installSupportAppMocks, SUPPORT_MESSAGES, WORKSPACE_SLUG } from '../fixtures/supportE2E'

for (const slowConversationRoute of [false, true]) {
  test(`Support stays mounted during first conversation selection (slow route: ${slowConversationRoute})`, async ({ page, baseURL }) => {
    test.setTimeout(90_000)
    await page.setViewportSize({ width: 1440, height: 900 })
    await installSupportAppMocks(page)
    if (slowConversationRoute) {
      await page.route(url => url.pathname.startsWith('/assets/_conversationId-') || (
        decodeURIComponent(url.pathname).endsWith('/$conversationId.tsx') && url.searchParams.has('tsr-split')
      ), async route => {
        await new Promise(resolve => setTimeout(resolve, 1000))
        await route.continue()
      })
    }

    await page.addInitScript(() => {
      let firstList: Element | null = null
      const state = { seen: false, recreated: false, disappeared: false }
      Object.assign(window, { __supportMountState: state })
      new MutationObserver(() => {
        const list = document.querySelector('[data-support-conversation-scroll]')
        if (!firstList && list?.getClientRects().length) {
          firstList = list
          state.seen = true
        }
        if (!firstList) return
        if (list && list !== firstList) state.recreated = true
        if (!firstList.isConnected || !firstList.getClientRects().length) state.disappeared = true
      }).observe(document, { childList: true, subtree: true, attributes: true, attributeFilter: ['style', 'hidden'] })
    })

    await page.goto(`${baseURL}/w/${WORKSPACE_SLUG}/support`, { waitUntil: 'domcontentloaded' })
    await expect(page).toHaveURL(new RegExp(`/support/${CONVERSATION_ID}`), { timeout: 60_000 })
    await expect(page.locator('[data-support-reply-composer] [contenteditable="true"]')).toBeVisible({ timeout: 20_000 })
    expect(await page.evaluate(() => (window as unknown as {
      __supportMountState: { seen: boolean; recreated: boolean; disappeared: boolean }
    }).__supportMountState)).toEqual({ seen: true, recreated: false, disappeared: false })
  })
}

test('conversation navigation preserves the inbox, filters, and each reply draft', async ({ page, baseURL }) => {
  test.setTimeout(90_000)
  await page.setViewportSize({ width: 1440, height: 900 })
  await installSupportAppMocks(page)
  const secondConversation = { ...buildConversation(), id: 'conv-2', customer_name: 'Second Visitor', subject: 'Second thread' }
  await page.route(url => url.pathname === '/api/support/inbox/conversations', route => route.fulfill({
    json: { data: [buildConversation(), secondConversation], total: 2, page: 1, per_page: 50, total_pages: 1 },
  }))
  await page.route(url => url.pathname.startsWith('/api/support/inbox/conversations/conv-2'), async route => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith('/conv-2')) return route.fulfill({ json: secondConversation })
    if (path.endsWith('/message-pages')) return route.fulfill({ json: {
      data: [{ ...SUPPORT_MESSAGES[0], id: 'msg-2', conversation_id: 'conv-2', content: 'Second conversation message' }], has_more: false,
    } })
    return route.fallback()
  })

  await page.goto(`${baseURL}/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}?sort=oldest`, { waitUntil: 'domcontentloaded' })
  const editor = page.locator('[data-support-reply-composer] [contenteditable="true"]')
  await expect(editor).toBeVisible({ timeout: 60_000 })
  const inbox = await page.locator('[data-support-conversation-scroll]').elementHandle()
  await editor.fill('Draft for the first conversation')
  await page.locator('[role="button"][data-conversation-id="conv-2"]').click()
  await expect(page).toHaveURL(/\/support\/conv-2\?sort=oldest/)
  await expect(page.locator('[data-support-message-list]')).toContainText('Second conversation message')
  await expect(editor).toBeEmpty()
  await editor.fill('Draft for the second conversation')
  await page.locator(`[role="button"][data-conversation-id="${CONVERSATION_ID}"]`).click()
  await expect(page).toHaveURL(new RegExp(`/support/${CONVERSATION_ID}\\?sort=oldest`))
  await expect(editor).toHaveText('Draft for the first conversation')
  expect(await inbox!.evaluate(node => node.isConnected)).toBe(true)

  await page.locator('[role="button"][data-conversation-id="conv-2"]').click()
  await expect(page).toHaveURL(/\/support\/conv-2\?sort=oldest/)
  await expect(editor).toHaveText('Draft for the second conversation')
  expect(await inbox!.evaluate(node => node.isConnected)).toBe(true)
})

test('Search and Coverage remain separate from the persistent inbox', async ({ page, baseURL }) => {
  test.setTimeout(90_000)
  await page.setViewportSize({ width: 1440, height: 900 })
  await installSupportAppMocks(page)
  await page.goto(`${baseURL}/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`, { waitUntil: 'domcontentloaded' })
  await expect(page.locator('[data-support-conversation-scroll]')).toBeVisible({ timeout: 60_000 })
  await page.getByRole('button', { name: 'Search conversations', exact: true }).click()
  await expect(page.getByPlaceholder('Search conversations by email, #number, title, customer, or message')).toBeVisible()
  await expect(page.locator('[data-support-conversation-scroll]')).toHaveCount(0)
  await page.getByRole('button', { name: 'Close search', exact: true }).click()
  await expect(page).toHaveURL(new RegExp(`/support/${CONVERSATION_ID}`))
  await expect(page.locator('[data-support-conversation-scroll]')).toBeVisible()

  await page.getByRole('button', { name: 'Coverage Gaps', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Coverage Gaps', exact: true })).toBeVisible()
  await expect(page.locator('[data-support-conversation-scroll]')).toHaveCount(0)
})
