import { describe, expect, it } from 'vitest'

import type { SupportMessageInfo } from '@/lib/pmTypes'
import { buildMessageInfoRows } from '../MessageInfoDialog'

const baseInfo: SupportMessageInfo = {
  id: 'msg-1',
  sent_at: '2026-06-04T07:13:51.193958Z',
  sender: {
    id: 'user-1',
    name: 'Waqar Azeem',
    type: 'user',
  },
  from: 'Operator',
  origin: 'widget',
  type: 'text',
  delivered: null,
  not_delivered_reason: null,
  read: false,
  read_at: null,
  edited: false,
  translated: false,
  automated: false,
}

describe('buildMessageInfoRows', () => {
  it('hides rows that are not relevant to a plain widget message', () => {
    const labels = buildMessageInfoRows(baseInfo).map(([label]) => label)

    expect(labels).toEqual(['Identifier', 'Created', 'Sender', 'Origin'])
  })

  it('renders a single email status row for delivery state and failure reason', () => {
    const rows = buildMessageInfoRows({
      ...baseInfo,
      from: 'support@example.com',
      email_delivery_status: 'bounced',
      email_delivery_status_label: 'Delivery failed',
      not_delivered_reason: 'Mailbox unavailable',
    })

    expect(rows).toContainEqual(['From', 'support@example.com'])
    expect(rows).toContainEqual(['Email status', 'Delivery failed · Mailbox unavailable'])
    expect(rows.map(([label]) => label)).not.toContain('Delivered')
    expect(rows.map(([label]) => label)).not.toContain('Not delivered')
  })

  it('renders email recipients for messages with cc and bcc metadata', () => {
    const rows = buildMessageInfoRows({
      ...baseInfo,
      from: 'support@example.com',
      to_email: 'customer@example.com',
      cc_emails: ['finance@example.com', 'manager@example.com'],
      bcc_emails: ['audit@example.com'],
    })

    expect(rows).toContainEqual(['From', 'support@example.com'])
    expect(rows).toContainEqual(['To', 'customer@example.com'])
    expect(rows).toContainEqual(['Cc', 'finance@example.com, manager@example.com'])
    expect(rows).toContainEqual(['Bcc', 'audit@example.com'])
  })
})
