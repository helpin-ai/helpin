import { describe, expect, it } from 'vitest'

import type { SupportMessage } from '@/lib/pmTypes'
import {
  appendOptimisticSupportMessage,
  buildOptimisticSupportMessage,
  reconcileOptimisticSupportMessage,
} from '../useSupport'

describe('support optimistic messages', () => {
  it('marks an immediate email reply as email-channel while delivery is pending', () => {
    const message = buildOptimisticSupportMessage({
      workspaceId: 'ws-1',
      conversationId: 'conv-1',
      payload: { content: 'We will check this.', is_internal: false, channels: ['email'] },
      user: {
        id: 'user-1',
        full_name: 'Waqar Azeem',
        email: 'waqar@example.com',
        avatar_url: 'https://example.com/avatar.png',
      },
      now: '2026-06-04T08:30:00.000Z',
      optimisticId: 'optimistic-1',
    })

    expect(message).toEqual(expect.objectContaining({
      id: 'optimistic-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'user-1',
      sender_display_name: 'Waqar Azeem',
      sender_avatar_url: 'https://example.com/avatar.png',
      content: 'We will check this.',
      is_internal: false,
      message_type: 'reply',
      via_channel: 'email',
      created_at: '2026-06-04T08:30:00.000Z',
      updated_at: '2026-06-04T08:30:00.000Z',
    }))
    expect(message.cancellable_until).toBeUndefined()
    expect(message.email_notified_at).toBeUndefined()
  })

  it('replaces the optimistic bubble with the persisted message', () => {
    const optimistic: SupportMessage = buildOptimisticSupportMessage({
      workspaceId: 'ws-1',
      conversationId: 'conv-1',
      payload: { content: 'Checking.' },
      user: { id: 'user-1', full_name: 'Agent', email: 'agent@example.com' },
      now: '2026-06-04T08:30:00.000Z',
      optimisticId: 'optimistic-1',
    })
    const persisted: SupportMessage = {
      ...optimistic,
      id: 'real-1',
      cancellable_until: '2026-06-04T08:32:00.000Z',
    }

    const appended = appendOptimisticSupportMessage([], optimistic)
    expect(reconcileOptimisticSupportMessage(appended, optimistic.id, persisted)).toEqual([persisted])
  })
})
