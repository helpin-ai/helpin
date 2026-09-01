import { expect, test } from '@playwright/test'
import { AGENT_ALICE, CONVERSATION_ID, installSupportAppMocks, WORKSPACE_SLUG } from '../fixtures/supportE2E'

declare global {
  interface Window {
    __supportE2E: {
      clearSentMessages: () => void
      disconnect: () => void
      emit: (payload: unknown) => void
      getSentMessages: () => Array<{ type: string; data?: Record<string, unknown> }>
      getSocketCount: () => number
      isReady: boolean
    }
  }
}

async function waitForSupportApp(page: import('@playwright/test').Page) {
  await page.waitForFunction(() => window.__supportE2E?.isReady === true)
  await expect(page.getByRole('button', { name: /Visitor Example/ })).toBeVisible()
}

function conversationRow(page: import('@playwright/test').Page) {
  return page.getByRole('button', { name: /Visitor Example/ })
}

test('real support route shows teammate typing and resyncs presence after reconnect', async ({ page, baseURL }) => {
  await installSupportAppMocks(page)
  await page.setViewportSize({ width: 1200, height: 900 })

  await page.goto(`${baseURL}/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`)
  await waitForSupportApp(page)

  await page.evaluate(() => window.__supportE2E.clearSentMessages())

  await page.evaluate(() => {
    window.__supportE2E.emit({
      action: 'typing_started',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'user-a',
      data: {
        content: 'Need another minute',
        agent_name: 'Alice Agent',
        agent_avatar: 'https://example.com/alice.png',
      },
    })
  })

  await expect(page.getByRole('button', { name: /Alice is typing…/ })).toBeVisible()
  await expect(page.getByText(/Alice Agent is typing/)).toBeVisible()
  await expect(page.locator(`img[alt="${AGENT_ALICE.name}"]`).first()).toBeVisible()

  await page.evaluate(() => {
    window.__supportE2E.clearSentMessages()
    window.__supportE2E.disconnect()
  })

  await page.waitForFunction(() => window.__supportE2E.getSocketCount() > 1)

  await expect.poll(async () => {
    return page.evaluate(() => window.__supportE2E.getSentMessages())
  }).toEqual(expect.arrayContaining([
    {
      type: 'support:presence:sync',
      data: expect.objectContaining({
        conversation_ids: expect.arrayContaining(['conv-1']),
      }),
    },
    {
      type: 'support:viewing:start',
      data: { conversation_id: 'conv-1' },
    },
  ]))
})

test('real support route keeps unread ahead of passive viewing and lets customer typing override agent typing', async ({ page, baseURL }) => {
  await installSupportAppMocks(page, { unreadCount: 3 })
  await page.setViewportSize({ width: 1200, height: 900 })

  await page.goto(`${baseURL}/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`)
  await waitForSupportApp(page)

  await page.evaluate(() => {
    window.__supportE2E.emit({
      action: 'viewing_started',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'user-a',
      data: {
        agent_name: 'Alice Agent',
        agent_avatar: 'https://example.com/alice.png',
      },
    })
  })

  const row = conversationRow(page)
  await expect(row).toContainText('3')
  await expect(row.locator(`img[alt="${AGENT_ALICE.name}"]`)).toHaveCount(0)

  await page.evaluate(() => {
    window.__supportE2E.emit({
      action: 'typing_started',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'user-a',
      data: {
        content: 'Need another minute',
        agent_name: 'Alice Agent',
        agent_avatar: 'https://example.com/alice.png',
      },
    })
  })

  await expect(page.getByRole('button', { name: /Alice is typing…/ })).toBeVisible()

  await page.evaluate(() => {
    window.__supportE2E.emit({
      action: 'typing_started',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'widget:sess-1',
      data: {
        content: 'Customer is typing now',
      },
    })
  })

  await expect(page.getByRole('button', { name: /Customer is typing now/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /Alice is typing…/ })).toHaveCount(0)
})

test('real support route replaces stale presence from authoritative snapshots after reconnect', async ({ page, baseURL }) => {
  await installSupportAppMocks(page)
  await page.setViewportSize({ width: 1200, height: 900 })

  await page.goto(`${baseURL}/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`)
  await waitForSupportApp(page)

  await page.evaluate(() => {
    window.__supportE2E.emit({
      action: 'typing_started',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'user-a',
      data: {
        content: 'Old draft',
        agent_name: 'Alice Agent',
        agent_avatar: 'https://example.com/alice.png',
      },
    })
  })

  await expect(page.getByRole('button', { name: /Alice is typing…/ })).toBeVisible()

  await page.evaluate(() => {
    window.__supportE2E.clearSentMessages()
    window.__supportE2E.disconnect()
  })

  await page.waitForFunction(() => window.__supportE2E.getSocketCount() > 1)

  await page.evaluate(() => {
    window.__supportE2E.emit({
      type: 'support:presence_snapshot',
      data: {
        conversation_id: 'conv-1',
        viewers: [
          {
            user_id: 'user-a',
            name: 'Alice Agent',
            avatar: 'https://example.com/alice.png',
          },
        ],
        typers: {
          'user-c': {
            content: 'Fresh draft',
            name: 'Charlie Agent',
            avatar: 'https://example.com/charlie.png',
          },
        },
      },
    })
  })

  await expect(page.getByRole('button', { name: /Charlie is typing…/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /Alice is typing…/ })).toHaveCount(0)
})

test('mobile support keeps Inbox mounted and opens conversations and Details as full-screen sheets', async ({ page, baseURL }) => {
  test.setTimeout(60_000)
  await installSupportAppMocks(page)
  await page.setViewportSize({ width: 390, height: 844 })

  await page.goto(`${baseURL}/w/${WORKSPACE_SLUG}/support`)
  await page.waitForFunction(() => window.__supportE2E?.isReady === true)

  const row = conversationRow(page)
  await expect(row).toBeVisible()
  await expect(page).toHaveURL(new RegExp(`/w/${WORKSPACE_SLUG}/support(?:\\?.*)?$`))
  await expect(page.getByRole('button', { name: 'Return to Inbox' })).toHaveCount(0)

  await row.click()

  await expect(page).toHaveURL(new RegExp(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`))
  await expect(page.getByRole('button', { name: 'Return to Inbox' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Create task' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Resolve conversation' })).toBeVisible()

  await page.getByRole('button', { name: 'Open conversation details' }).click()
  await expect(page.getByRole('heading', { name: 'Details' })).toBeVisible()
  await page.getByRole('button', { name: 'Close conversation details' }).click()
  await expect(page.getByRole('heading', { name: 'Details' })).toHaveCount(0)

  await page.getByRole('button', { name: 'Return to Inbox' }).click()
  await expect(page).toHaveURL(new RegExp(`/w/${WORKSPACE_SLUG}/support(?:\\?.*)?$`))
  await expect(row).toBeVisible()
  await expect(row).toBeFocused()
})

test('mobile support direct links return to Inbox without relying on route history', async ({ page, baseURL }) => {
  test.setTimeout(60_000)
  await installSupportAppMocks(page)
  await page.setViewportSize({ width: 390, height: 844 })

  await page.goto(`${baseURL}/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`)
  await page.waitForFunction(() => window.__supportE2E?.isReady === true)
  await expect(page.getByRole('button', { name: 'Return to Inbox' })).toBeVisible()

  await page.getByRole('button', { name: 'Return to Inbox' }).click()

  await expect(page).toHaveURL(new RegExp(`/w/${WORKSPACE_SLUG}/support(?:\\?.*)?$`))
  await expect(conversationRow(page)).toBeVisible()
})
