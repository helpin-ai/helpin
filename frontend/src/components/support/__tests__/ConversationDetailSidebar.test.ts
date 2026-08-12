import { describe, expect, it, vi } from 'vitest'

import type { SupportConversation, SupportMessage } from '@/lib/pmTypes'
import {
  copyCustomerEmailToClipboard,
  conversationDetailSidebarRootClassName,
  customerEmailCopyButtonClassName,
  customerEmailDisplayRowClassName,
  customerNameDisplayRowClassName,
  customerNameEditButtonClassName,
  getAddConversationCCResult,
  getConversationEmailRecipients,
  getLatestEmailRecipients,
  getLastActiveTooltipLabel,
  shouldShowLastActiveIndicator,
} from '../ConversationDetailSidebar'

describe('details sidebar scrolling', () => {
  it('constrains the sidebar to its panel height so the inner content can scroll', () => {
    expect(conversationDetailSidebarRootClassName).toContain('h-full')
    expect(conversationDetailSidebarRootClassName).toContain('min-h-0')
    expect(conversationDetailSidebarRootClassName).not.toContain('border-l')
  })
})

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

describe('getConversationEmailRecipients', () => {
  it('keeps the primary recipient, reply cc, and also-on-thread buckets distinct', () => {
    const conversation = {
      customer_email: 'teammate@company.com',
      email_cc: ['jane@example.com'],
      email_thread_participants: ['jane@example.com', 'manager@example.com', 'teammate@company.com'],
    } as SupportConversation

    expect(getConversationEmailRecipients(conversation)).toEqual({
      primary: ['teammate@company.com'],
      cc: ['jane@example.com'],
      alsoOnThread: ['manager@example.com'],
    })
  })
})

describe('getAddConversationCCResult', () => {
  const recipients = {
    primary: ['brian@example.com'],
    cc: ['ops@example.com'],
    alsoOnThread: ['finance@example.com'],
  }

  it('normalizes a new cc email and appends it to the reply recipient list', () => {
    expect(getAddConversationCCResult(recipients, '  Sarah@Example.com  ')).toEqual({
      cc: ['ops@example.com', 'sarah@example.com'],
    })
  })

  it('allows a thread participant to be promoted to cc replies', () => {
    expect(getAddConversationCCResult(recipients, 'finance@example.com')).toEqual({
      cc: ['ops@example.com', 'finance@example.com'],
    })
  })

  it('rejects invalid, primary, and duplicate cc values', () => {
    expect(getAddConversationCCResult(recipients, 'not-an-email')).toEqual({
      cc: ['ops@example.com'],
      error: 'Enter a valid email address.',
    })
    expect(getAddConversationCCResult(recipients, 'BRIAN@example.com')).toEqual({
      cc: ['ops@example.com'],
      error: 'This is already the To recipient.',
    })
    expect(getAddConversationCCResult(recipients, 'OPS@example.com')).toEqual({
      cc: ['ops@example.com'],
      error: 'Already added to Cc.',
    })
  })
})

describe('last active presence helpers', () => {
  it('shows the amber indicator only for offline visitors with known activity', () => {
    expect(shouldShowLastActiveIndicator(false, '2026-06-16T10:00:00Z')).toBe(true)
    expect(shouldShowLastActiveIndicator(true, '2026-06-16T10:00:00Z')).toBe(false)
    expect(shouldShowLastActiveIndicator(false, null)).toBe(false)
  })

  it('keeps the last active label compact for sidebar display', () => {
    const label = getLastActiveTooltipLabel('2026-06-16T10:00:00Z', 'crm_contact', new Date('2026-06-16T10:12:00Z'))

    expect(label).toBe('Last active 12 minutes ago')
  })
})

describe('copyCustomerEmailToClipboard', () => {
  it('copies the trimmed email address', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)

    const copied = await copyCustomerEmailToClipboard('  buyer@example.com  ', writeText)

    expect(copied).toBe(true)
    expect(writeText).toHaveBeenCalledWith('buyer@example.com')
  })

  it('does not copy empty email values', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)

    const copied = await copyCustomerEmailToClipboard('   ', writeText)

    expect(copied).toBe(false)
    expect(writeText).not.toHaveBeenCalled()
  })
})

describe('customer name edit affordance', () => {
  it('keeps the pencil hidden until the name row is hovered or focused', () => {
    expect(customerNameEditButtonClassName).toContain('opacity-0')
    expect(customerNameEditButtonClassName).toContain('group-hover/name:opacity-100')
    expect(customerNameEditButtonClassName).toContain('group-focus-within/name:opacity-100')
  })

  it('keeps the customer name centered with equal left and right affordance columns', () => {
    expect(customerNameDisplayRowClassName).toContain('grid')
    expect(customerNameDisplayRowClassName).toContain('w-fit')
    expect(customerNameDisplayRowClassName).toContain('grid-cols-[1.5rem_minmax(0,1fr)_1.5rem]')
    expect(customerNameEditButtonClassName).toContain('col-start-3')
    expect(customerNameEditButtonClassName).not.toContain('absolute')
  })
})

describe('customer email copy affordance', () => {
  it('keeps the email centered with equal icon and copy columns', () => {
    expect(customerEmailDisplayRowClassName).toContain('grid')
    expect(customerEmailDisplayRowClassName).toContain('w-fit')
    expect(customerEmailDisplayRowClassName).toContain('grid-cols-[1.5rem_minmax(0,1fr)_1.5rem]')
    expect(customerEmailCopyButtonClassName).toContain('col-start-3')
    expect(customerEmailCopyButtonClassName).not.toContain('absolute')
  })

  it('keeps the copy button hidden until the email row is hovered or focused', () => {
    expect(customerEmailCopyButtonClassName).toContain('opacity-0')
    expect(customerEmailCopyButtonClassName).toContain('group-hover/email:opacity-100')
    expect(customerEmailCopyButtonClassName).toContain('group-focus-within/email:opacity-100')
  })
})
