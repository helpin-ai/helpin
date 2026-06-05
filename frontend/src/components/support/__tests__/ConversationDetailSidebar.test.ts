import { describe, expect, it } from 'vitest'

import type { SupportMessage } from '@/lib/pmTypes'
import { getLatestEmailRecipients } from '../ConversationDetailSidebar'

const baseMessage: SupportMessage = {
  id: 'msg-1',
  workspace_id: 'ws-1',
  conversation_id: 'conv-1',
  sender_type: 'user',
  sender_display_name: 'Operator',
  content: 'Hello',
  is_internal: false,
  created_at: '2026-06-04T07:00:00.000Z',
  updated_at: '2026-06-04T07:00:00.000Z',
}

describe('getLatestEmailRecipients', () => {
  it('uses the latest non-internal outbound message with email recipients', () => {
    const recipients = getLatestEmailRecipients([
      {
        ...baseMessage,
        id: 'msg-1',
        created_at: '2026-06-04T07:00:00.000Z',
        email_to: 'old@example.com',
      },
      {
        ...baseMessage,
        id: 'msg-2',
        created_at: '2026-06-04T08:00:00.000Z',
        email_to: 'customer@example.com',
        email_cc: ['finance@example.com'],
        email_bcc: ['audit@example.com'],
      },
    ])

    expect(recipients).toEqual({
      to: 'customer@example.com',
      cc: ['finance@example.com'],
      bcc: ['audit@example.com'],
    })
  })

  it('ignores customer and internal messages', () => {
    const recipients = getLatestEmailRecipients([
      {
        ...baseMessage,
        sender_type: 'customer',
        email_to: 'support@example.com',
      },
      {
        ...baseMessage,
        sender_type: 'user',
        is_internal: true,
        email_to: 'private@example.com',
      },
    ])

    expect(recipients).toBeNull()
  })
})
