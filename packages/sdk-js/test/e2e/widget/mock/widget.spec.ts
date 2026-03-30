import { expect, test, type Page } from '@playwright/test'
import {
  CONVERSATION_ID,
  LINK_PREVIEW_URL,
  WIDGET_HOST,
  WIDGET_KEY,
  installWidgetMocks,
} from './widgetE2E'

declare global {
  interface Window {
    helpin?: (...args: unknown[]) => unknown
    __widgetE2E?: {
      clearRequests: () => void
      clearSentMessages: () => void
      disconnect: () => void
      emit: (payload: unknown) => void
      getRequests: () => Array<{ method: string; url: string; body?: Record<string, unknown> }>
      getSentMessages: () => Array<{ type: string; data?: Record<string, unknown> }>
      getSocketCount: () => number
      isReady: boolean
    }
  }
}

async function bootWidget(page: Page, options: { unreadCount?: number; persistedSession?: boolean; invalidStoredSession?: boolean } = {}) {
  await installWidgetMocks(page, options)
  await page.goto('/test/e2e/widget/mock/test-page.html')
  await page.waitForFunction(() => typeof window.helpin === 'function')

  await page.evaluate(({ key, host }) => {
    window.helpin?.('boot', { key, host })
  }, { key: WIDGET_KEY, host: WIDGET_HOST })

  await expect.poll(async () => {
    return page.evaluate(() => window.helpin?.('isWidgetReady') === true)
  }).toBe(true)

  await expect(page.locator('.helpin-launcher')).toBeVisible()
}

async function openWidget(page: Page) {
  await page.evaluate(() => {
    window.helpin?.('show')
  })
  await expect(page.locator('.helpin-chat-window')).toBeVisible()
}

test('boots the widget and renders the active teammate conversation view', async ({ page }) => {
  await bootWidget(page)
  await openWidget(page)

  await expect(page.locator('.helpin-conversation-title')).toContainText('Alice Agent')
  await expect(page.locator('.helpin-message-list')).toContainText('Initial message')
})

test('shows unread state on the launcher and in the messages list', async ({ page }) => {
  await bootWidget(page, { unreadCount: 2 })

  await expect(page.locator('.helpin-unread-badge')).toHaveText('2')

  await page.evaluate(() => {
    window.helpin?.('showMessages')
  })

  await expect(page.locator('.helpin-conversation-item')).toContainText('Initial message')
  await expect(page.locator('.helpin-conversation-item-badge')).toHaveText('2')
})

test('renders a clickable bare URL and a link preview when the customer sends a link', async ({ page }) => {
  await bootWidget(page)
  await openWidget(page)

  await page.locator('.helpin-compose-input').fill(LINK_PREVIEW_URL)
  await page.locator('.helpin-compose-send').click()

  await expect(page.locator(`.helpin-message-content a[href="${LINK_PREVIEW_URL}"]`).last()).toBeVisible()
  await expect(page.locator('.helpin-link-preview-title').last()).toHaveText('Example Docs')

  await expect.poll(async () => {
    return page.evaluate(() => window.__widgetE2E?.getSentMessages() || [])
  }).toEqual(expect.arrayContaining([
    {
      type: 'message:send',
      data: { content: LINK_PREVIEW_URL },
    },
  ]))
})

test('shows agent typing identity and clears it on typing stop', async ({ page }) => {
  await bootWidget(page)
  await openWidget(page)

  await page.evaluate(() => {
    window.__widgetE2E?.emit({
      type: 'typing:start',
      data: {
        agent_name: 'Alice Agent',
        agent_avatar: 'https://example.com/alice.png',
      },
    })
  })

  const typingIndicator = page.locator('.helpin-typing-indicator')
  await expect(typingIndicator).toBeVisible()
  await expect(typingIndicator).toContainText('Alice Agent is typing...')
  await expect(typingIndicator.locator('img[alt="Alice Agent"]')).toBeVisible()

  await page.evaluate(() => {
    window.__widgetE2E?.emit({
      type: 'typing:stop',
      data: {},
    })
  })

  await expect(page.locator('.helpin-typing-indicator')).toHaveCount(0)
})

test('requests a transcript for the active conversation from the conversation menu', async ({ page }) => {
  await bootWidget(page)
  await openWidget(page)

  await page.evaluate(() => {
    window.__widgetE2E?.clearRequests()
  })

  await page.locator('.helpin-conversation-header-btn').click()
  await page.getByRole('button', { name: 'Transcript' }).click()
  await page.locator('.helpin-conversation-transcript-input').fill('visitor@example.com')
  await page.getByRole('button', { name: 'Send transcript' }).click()

  await expect.poll(async () => {
    return page.evaluate(() => window.__widgetE2E?.getRequests() || [])
  }).toEqual(expect.arrayContaining([
    {
      method: 'POST',
      url: `https://${WIDGET_HOST}/widget/conversations/${CONVERSATION_ID}/transcript`,
      body: {
        session_token: 'session-1',
        email: 'visitor@example.com',
      },
    },
  ]))

  await expect(page.locator('.helpin-conversation-transcript-status')).toContainText('Transcript sent to visitor@example.com')
})

test('renders inbound agent messages and updates the launcher unread badge while hidden', async ({ page }) => {
  await bootWidget(page)

  await page.evaluate(() => {
    window.__widgetE2E?.emit({
      type: 'message:received',
      data: {
        id: 'msg-agent-1',
        conversation_id: 'conv-1',
        sender_type: 'agent',
        sender_name: 'Alice Agent',
        sender_avatar: 'https://example.com/alice.png',
        content: 'We have an update for you.',
        created_at: '2026-03-27T20:05:00Z',
      },
    })
  })

  await expect(page.locator('.helpin-unread-badge')).toHaveText('1')

  await openWidget(page)
  await expect(page.locator('.helpin-message-list')).toContainText('We have an update for you.')
})

test('restores a stored widget session on boot and again after reconnect', async ({ page }) => {
  await bootWidget(page, { persistedSession: true })
  await openWidget(page)

  await expect.poll(async () => {
    return page.evaluate(() => window.__widgetE2E?.getSentMessages() || [])
  }).toEqual(expect.arrayContaining([
    {
      type: 'session:restore',
      data: { session_token: 'session-1' },
    },
  ]))

  await page.evaluate(() => {
    window.__widgetE2E?.clearSentMessages()
    window.__widgetE2E?.disconnect()
  })

  await page.waitForFunction(() => (window.__widgetE2E?.getSocketCount() || 0) > 1)

  await expect.poll(async () => {
    return page.evaluate(() => window.__widgetE2E?.getSentMessages() || [])
  }, { timeout: 5_000 }).toEqual(expect.arrayContaining([
    {
      type: 'session:restore',
      data: { session_token: 'session-1' },
    },
  ]))

  await expect(page.locator('.helpin-message-list')).toContainText('Initial message')
})

test('falls back to session:create when a stored session is rejected', async ({ page }) => {
  await bootWidget(page, { invalidStoredSession: true })
  await openWidget(page)

  await expect.poll(async () => {
    return page.evaluate(() => window.__widgetE2E?.getSentMessages() || [])
  }).toEqual(expect.arrayContaining([
    {
      type: 'session:restore',
      data: { session_token: 'session-1' },
    },
    {
      type: 'session:create',
      data: expect.objectContaining({
        anonymous_id: expect.any(String),
      }),
    },
  ]))

  await expect(page.locator('.helpin-message-list')).toContainText('Initial message')
})

test('uploads an attachment and sends it with the customer message', async ({ page }) => {
  await bootWidget(page)
  await openWidget(page)

  await page.evaluate(() => {
    window.__widgetE2E?.clearRequests()
    window.__widgetE2E?.clearSentMessages()
  })

  await page.locator('input[type="file"]').setInputFiles({
    name: 'invoice.txt',
    mimeType: 'text/plain',
    buffer: Buffer.from('hello from attachment'),
  })

  await expect(page.locator('.helpin-compose-attachment-filename')).toHaveText('invoice.txt')
  await expect(page.locator('.helpin-compose-attachment-progress')).toHaveCount(0)

  await page.locator('.helpin-compose-send').click()

  await expect.poll(async () => {
    return page.evaluate(() => window.__widgetE2E?.getRequests() || [])
  }).toEqual(expect.arrayContaining([
    {
      method: 'POST',
      url: `https://${WIDGET_HOST}/widget/support/attachments`,
      body: {
        file_name: 'invoice.txt',
        file_size: 21,
        content_type: 'text/plain',
      },
    },
    {
      method: 'PATCH',
      url: `https://${WIDGET_HOST}/widget/support/attachments/att-1/confirm`,
    },
  ]))

  await expect.poll(async () => {
    return page.evaluate(() => window.__widgetE2E?.getSentMessages() || [])
  }).toEqual(expect.arrayContaining([
    {
      type: 'message:send',
      data: {
        content: '',
        attachment_ids: ['att-1'],
      },
    },
  ]))

  await expect(page.locator('.helpin-attachment-file')).toContainText('invoice.txt')
})
