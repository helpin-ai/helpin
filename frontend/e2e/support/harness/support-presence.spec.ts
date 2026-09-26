import { expect, test } from '@playwright/test'
import { AGENT_ALICE, AGENT_CHARLIE } from '../fixtures/supportE2E'

declare global {
  interface Window {
    __supportPresenceHarness: {
      clearSentMessages: () => void
      disconnect: () => void
      emit: (payload: unknown) => void
      getSentMessages: () => Array<{ type: string; data?: Record<string, unknown> }>
      getSocketCount: () => number
      isReady: boolean
      sendTypingStart: (content: string) => void
      sendTypingStop: () => void
    }
  }
}

async function waitForHarness(page: import('@playwright/test').Page) {
  await page.waitForFunction(() => window.__supportPresenceHarness?.isReady === true)
}

function conversationRow(page: import('@playwright/test').Page) {
  return page.locator('[data-conversation-id="conv-1"][role="button"]')
}

test('agent typing is visible to another agent in list and thread', async ({ browser, baseURL }) => {
  const agentA = await browser.newPage()
  const agentB = await browser.newPage()

  await agentA.goto(`${baseURL}/e2e/support/harness/support-presence-harness.html?userId=user-a&name=Alice%20Agent`)
  await agentB.goto(`${baseURL}/e2e/support/harness/support-presence-harness.html?userId=user-b&name=Bob%20Agent`)

  await waitForHarness(agentA)
  await waitForHarness(agentB)

  await agentA.evaluate(() => {
    window.__supportPresenceHarness.clearSentMessages()
    window.__supportPresenceHarness.sendTypingStart('Draft reply')
  })

  await expect.poll(async () => {
    return agentA.evaluate(() => window.__supportPresenceHarness.getSentMessages())
  }).toContainEqual({
    type: 'support:typing:start',
    data: { conversation_id: 'conv-1', content: 'Draft reply' },
  })

  await agentB.evaluate(() => {
    window.__supportPresenceHarness.emit({
      action: 'typing_started',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'user-a',
      data: {
        content: 'Draft reply',
        agent_name: 'Alice Agent',
        agent_avatar: 'https://example.com/alice.png',
      },
    })
  })

  await expect(agentB.getByText('Alice is typing…')).toBeVisible()
  await expect(agentB.locator(`img[alt="${AGENT_ALICE.name}"]`)).toBeVisible()
  await expect(agentB.getByTestId('thread-agent-typing')).toHaveText(AGENT_ALICE.name)

  await agentA.close()
  await agentB.close()
})

test('multiple agents typing stay visible together in the list and thread', async ({ page, baseURL }) => {
  await page.goto(`${baseURL}/e2e/support/harness/support-presence-harness.html?userId=user-b&name=Bob%20Agent`)
  await waitForHarness(page)

  await page.evaluate(() => {
    window.__supportPresenceHarness.emit({
      action: 'typing_started',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'user-a',
      data: {
        content: 'Draft reply',
        agent_name: 'Alice Agent',
        agent_avatar: 'https://example.com/alice.png',
      },
    })
    window.__supportPresenceHarness.emit({
      action: 'typing_started',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'user-c',
      data: {
        content: 'Another draft',
        agent_name: 'Charlie Agent',
        agent_avatar: 'https://example.com/charlie.png',
      },
    })
  })

  await expect(page.getByText('Alice is typing…')).toBeVisible()
  await expect(page.locator(`img[alt="${AGENT_ALICE.name}"]`)).toBeVisible()
  await expect(page.locator(`img[alt="${AGENT_CHARLIE.name}"]`)).toBeVisible()
  await expect(page.getByTestId('thread-agent-typing')).toHaveText(`${AGENT_ALICE.name}, ${AGENT_CHARLIE.name}`)
})

test('customer typing takes precedence over agent typing and unread state in the conversation list', async ({ page, baseURL }) => {
  await page.goto(`${baseURL}/e2e/support/harness/support-presence-harness.html?userId=user-b&name=Bob%20Agent&unreadCount=3`)
  await waitForHarness(page)

  await page.evaluate(() => {
    window.__supportPresenceHarness.emit({
      action: 'typing_started',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'user-a',
      data: {
        content: 'Draft reply',
        agent_name: 'Alice Agent',
        agent_avatar: 'https://example.com/alice.png',
      },
    })
  })

  await expect(page.getByRole('button', { name: /Alice is typing…/ })).toBeVisible()

  await page.evaluate(() => {
    window.__supportPresenceHarness.emit({
      action: 'typing_started',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'widget:sess-1',
      data: {
        content: 'Customer typing here',
      },
    })
  })

  await expect(page.getByRole('button', { name: /Customer typing here/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /Alice is typing…/ })).toHaveCount(0)
})

test('unread state still wins over passive teammate viewing in the conversation list', async ({ page, baseURL }) => {
  await page.goto(`${baseURL}/e2e/support/harness/support-presence-harness.html?userId=user-b&name=Bob%20Agent&unreadCount=4`)
  await waitForHarness(page)

  await page.evaluate(() => {
    window.__supportPresenceHarness.emit({
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
  await expect(row).toContainText('4')
  await expect(row.locator(`img[alt="${AGENT_ALICE.name}"]`)).toHaveCount(0)
})

test('passive teammate viewing avatars are shown when there is no typing or unread state', async ({ page, baseURL }) => {
  await page.goto(`${baseURL}/e2e/support/harness/support-presence-harness.html?userId=user-b&name=Bob%20Agent`)
  await waitForHarness(page)

  await page.evaluate(() => {
    window.__supportPresenceHarness.emit({
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
  await expect(row.locator(`img[alt="${AGENT_ALICE.name}"]`)).toBeVisible()
  await expect(page.getByText('Alice is typing…')).toHaveCount(0)
})

test('when agent typing stops the row falls back to passive viewing avatars', async ({ page, baseURL }) => {
  await page.goto(`${baseURL}/e2e/support/harness/support-presence-harness.html?userId=user-b&name=Bob%20Agent`)
  await waitForHarness(page)

  await page.evaluate(() => {
    window.__supportPresenceHarness.emit({
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
    window.__supportPresenceHarness.emit({
      action: 'typing_started',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'user-a',
      data: {
        content: 'Draft reply',
        agent_name: 'Alice Agent',
        agent_avatar: 'https://example.com/alice.png',
      },
    })
  })

  await expect(page.getByText('Alice is typing…')).toBeVisible()

  await page.evaluate(() => {
    window.__supportPresenceHarness.emit({
      action: 'typing_stopped',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'user-a',
      data: {},
    })
  })

  const row = conversationRow(page)
  await expect(page.getByText('Alice is typing…')).toHaveCount(0)
  await expect(row.locator(`img[alt="${AGENT_ALICE.name}"]`)).toBeVisible()
})

test('reconnect triggers presence resync for the list and active thread', async ({ page, baseURL }) => {
  await page.goto(`${baseURL}/e2e/support/harness/support-presence-harness.html?userId=user-b&name=Bob%20Agent`)
  await waitForHarness(page)

  await page.evaluate(() => window.__supportPresenceHarness.clearSentMessages())
  await page.evaluate(() => window.__supportPresenceHarness.disconnect())

  await page.waitForFunction(() => window.__supportPresenceHarness.getSocketCount() > 1)

  await expect.poll(async () => {
    return page.evaluate(() => window.__supportPresenceHarness.getSentMessages())
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

test('presence snapshots replace stale typing and viewing state after reconnect', async ({ page, baseURL }) => {
  await page.goto(`${baseURL}/e2e/support/harness/support-presence-harness.html?userId=user-b&name=Bob%20Agent`)
  await waitForHarness(page)

  await page.evaluate(() => {
    window.__supportPresenceHarness.emit({
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
    window.__supportPresenceHarness.emit({
      action: 'viewing_started',
      entity: 'support_conversation',
      entity_id: 'conv-1',
      workspace_id: 'ws-1',
      actor_id: 'user-c',
      data: {
        agent_name: 'Charlie Agent',
        agent_avatar: 'https://example.com/charlie.png',
      },
    })
  })

  await expect(page.getByTestId('thread-agent-typing')).toHaveText(AGENT_ALICE.name)
  await expect(page.getByTestId('thread-viewers')).toHaveText('user-c')

  await page.evaluate(() => {
    window.__supportPresenceHarness.clearSentMessages()
    window.__supportPresenceHarness.disconnect()
  })

  await page.waitForFunction(() => window.__supportPresenceHarness.getSocketCount() > 1)

  await page.evaluate(() => {
    window.__supportPresenceHarness.emit({
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
  await expect(page.getByTestId('thread-agent-typing')).toHaveText(AGENT_CHARLIE.name)
  await expect(page.getByTestId('thread-viewers')).toHaveText('user-a')
})
