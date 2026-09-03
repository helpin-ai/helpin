import { describe, expect, it } from 'vitest'

import type { SupportMessage } from '@/lib/pmTypes'
import { getSupportReceiptStatus } from '../helpers'

const externalMessage: SupportMessage = {
  id: 'msg-external-email-1',
  workspace_id: 'ws-1',
  conversation_id: 'conv-1',
  sender_type: 'user',
  sender_user_id: 'agent-1',
  sender_display_name: 'Agent',
  content: 'Sent directly from Gmail.',
  message_type: 'reply',
  is_internal: false,
  metadata: '{"external_email_reply":true,"external_email_capture":"support_email_copy"}',
  via_channel: 'email',
  email_delivery_status: 'delivered',
  email_read_at: '2026-09-03T07:36:00.000Z',
  created_at: '2026-09-03T07:35:00.000Z',
  updated_at: '2026-09-03T07:36:00.000Z',
}

describe('getSupportReceiptStatus', () => {
  it('never claims Helpin delivery or read tracking for copied teammate email', () => {
    expect(getSupportReceiptStatus(externalMessage, {
      source: 'email',
      contact_last_seen_at: '2026-09-03T07:37:00.000Z',
    })).toBe('sent_outside_helpin')
  })

  it('preserves tracked Helpin email receipts', () => {
    expect(getSupportReceiptStatus({
      ...externalMessage,
      id: 'msg-helpin-email-1',
      metadata: undefined,
    }, {
      source: 'email',
      contact_last_seen_at: undefined,
    })).toBe('read_email')
  })
})
