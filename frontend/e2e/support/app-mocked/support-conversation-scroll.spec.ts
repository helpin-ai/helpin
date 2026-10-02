import { expect, test } from '@playwright/test'
import {
  buildConversation,
  installSupportAppMocks,
  SUPPORT_MESSAGES,
  WORKSPACE_SLUG,
} from '../fixtures/supportE2E'

// Cover both the bottom of the list and a position with more rows below it.
for (const height of [900, 600]) {
  test(`keeps the list in place after replying to conversation 25 of 30 (${height}px)`, async ({ page }, testInfo) => {
    test.setTimeout(150_000)
    await page.setViewportSize({ width: 1440, height })
    await installSupportAppMocks(page)
    const conversations = Array.from({ length: 30 }, (_, i) => ({
      ...buildConversation(),
      id: `conv-${i + 1}`,
      display_id: i + 1,
      customer_name: `Customer ${String(i + 1).padStart(2, '0')}`,
      subject: `Conversation ${i + 1}`,
      list_last_activity_at: new Date(Date.UTC(2026, 8, 30, 12) - i * 3_600_000).toISOString(),
    }))
    const replies = new Map<string, Record<string, unknown>[]>()
    await page.route('**/api/support/inbox/conversations**', async route => {
      const url = new URL(route.request().url())
      const match = url.pathname.match(/\/conversations(?:\/(conv-\d+))?(?:\/(.*))?$/)
      if (!match) return route.fallback()
      const [, id, action] = match
      if (!id) {
        const data = [...conversations].sort((a, b) => b.list_last_activity_at.localeCompare(a.list_last_activity_at))
        return route.fulfill({ json: { data, total: 30, page: 1, per_page: 50, total_pages: 1 } })
      }
      const conversation = conversations.find(c => c.id === id)
      if (!conversation) return route.fallback()
      if (!action) return route.fulfill({ json: conversation })
      if ((action === 'messages' || action === 'translation/sends') && route.request().method() === 'POST') {
        conversation.list_last_activity_at = '2026-10-01T12:00:00Z'
        const message = {
          id: `reply-${id}`,
          workspace_id: 'ws-1',
          conversation_id: id,
          sender_type: 'user',
          sender_user_id: 'user-b',
          message_type: 'reply',
          ...route.request().postDataJSON(),
          created_at: conversation.list_last_activity_at,
        }
        replies.set(id, [message])
        return route.fulfill({ json: message })
      }
      const messages = [
        ...SUPPORT_MESSAGES.map(m => ({ ...m, id: `message-${id}`, conversation_id: id })),
        ...(replies.get(id) ?? []),
      ]
      if (action === 'message-pages') return route.fulfill({ json: { data: messages, has_more: false } })
      if (action === 'messages') return route.fulfill({ json: messages })
      if (action === 'translation') {
        return route.fulfill({ json: {
          available: false,
          languages: { en: 'English' },
          preference: { reading_language: 'en' },
          conversation: { translation_mode: 'inherit' },
        } })
      }
      return route.fallback()
    })
    await page.goto(`/w/${WORKSPACE_SLUG}/support`)
    const list = page.locator('[data-support-conversation-scroll]:visible')
    await expect(list.locator('[data-conversation-id]')).toHaveCount(30, { timeout: 90_000 })
    const snapshot = () => list.evaluate(el => {
      const bounds = el.getBoundingClientRect()
      return {
        scrollTop: el.scrollTop,
        height: el.clientHeight,
        scrollHeight: el.scrollHeight,
        visible: [...el.querySelectorAll('[data-conversation-id]')].filter(row => {
          const rect = row.getBoundingClientRect()
          return rect.bottom > bounds.top && rect.top < bounds.bottom
        }).map(row => ({
          id: row.getAttribute('data-conversation-id'),
          top: Math.round(row.getBoundingClientRect().top - bounds.top),
        })),
      }
    })
    await list.locator('[data-conversation-id="conv-25"]').scrollIntoViewIfNeeded()
    await list.locator('[data-conversation-id="conv-25"]').click()
    const composer = page.locator('[data-support-reply-composer]:visible')
    await expect(composer.locator('[contenteditable="true"]')).toBeVisible()
    await composer.locator('[contenteditable="true"]').fill('Following up on conversation 25')
    const before = await snapshot()
    await page.screenshot({ path: testInfo.outputPath('before.png') })
    await composer.getByRole('button', { name: 'Send', exact: true }).click()
    await expect.poll(() => replies.has('conv-25')).toBe(true)
    await expect(list.locator('[data-conversation-id]').first()).toHaveAttribute('data-conversation-id', 'conv-25')
    await expect(composer.locator('[contenteditable="true"]')).toBeEmpty()
    await page.screenshot({ path: testInfo.outputPath('after.png') })
    const after = await snapshot()
    await testInfo.attach('scroll-measurements', {
      body: JSON.stringify({ before, after, url: page.url() }, null, 2),
      contentType: 'application/json',
    })
    expect(before.scrollTop).toBeGreaterThan(0)
    expect(after.visible.some(row => row.id === 'conv-26')).toBe(true)
    expect(after.scrollTop).toBe(before.scrollTop)
    expect(after.visible.find(row => row.id === 'conv-26')?.top)
      .toBe(before.visible.find(row => row.id === 'conv-26')?.top)
    await expect(page).toHaveURL(/support\/conv-25$/)
    // Continue to the next conversation without scrolling back down.
    await list.locator('[data-conversation-id="conv-26"]').click()
    await expect(page).toHaveURL(/support\/conv-26$/)
    expect((await snapshot()).scrollTop).toBe(before.scrollTop)
    // Intentional scrolling still works; the replied conversation is first.
    await list.evaluate(el => { el.scrollTop = 0 })
    await expect(list.locator('[data-conversation-id="conv-25"]')).toBeInViewport()
  })
}
