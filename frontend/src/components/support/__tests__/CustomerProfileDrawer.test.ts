import { describe, expect, it } from 'vitest'

import type { CRMContact } from '@/lib/crmTypes'
import type { SupportConversation } from '@/lib/pmTypes'
import {
  customerProfileInitialContactValues,
  customerProfilePanelClassName,
  customerProfileDisplayName,
  splitCustomerDisplayName,
} from '../CustomerProfileDrawer'

const conversation = {
  id: 'conv-1',
  workspace_id: 'ws-1',
  display_id: 1,
  subject: 'Question',
  status: 'open',
  priority: 'medium',
  source: 'email',
  customer_name: 'Fallback Name',
  customer_email: 'fallback@example.com',
  created_at: '2026-07-02T00:00:00.000Z',
  updated_at: '2026-07-02T00:00:00.000Z',
} satisfies SupportConversation

const contact = {
  id: 'contact-1',
  workspace_id: 'ws-1',
  display_id: 'CON-1',
  first_name: 'Ada',
  last_name: 'Lovelace',
  email: 'ada@example.com',
  lifecycle_stage: 'lead',
  lead_status: 'open',
  custom_properties: {},
  created_at: '2026-07-02T00:00:00.000Z',
  updated_at: '2026-07-02T00:00:00.000Z',
} satisfies CRMContact

describe('customerProfileDisplayName', () => {
  it('prefers the linked CRM contact name', () => {
    expect(customerProfileDisplayName(contact, conversation)).toBe('Ada Lovelace')
  })

  it('falls back to the support conversation identity', () => {
    expect(customerProfileDisplayName(null, conversation)).toBe('Fallback Name')
    expect(customerProfileDisplayName(null, { ...conversation, customer_name: undefined })).toBe('fallback@example.com')
  })
})

describe('splitCustomerDisplayName', () => {
  it('splits display names for contact creation or updates', () => {
    expect(splitCustomerDisplayName('  Ada   Lovelace  ')).toEqual({ firstName: 'Ada', lastName: 'Lovelace' })
    expect(splitCustomerDisplayName('Ada')).toEqual({ firstName: 'Ada', lastName: undefined })
  })
})

describe('customer profile panel', () => {
  it('uses subtle slide and fade animation classes', () => {
    expect(customerProfilePanelClassName).toContain('transition-all')
    expect(customerProfilePanelClassName).toContain('duration-200')
    expect(customerProfilePanelClassName).toContain('translate-x-0')
    expect(customerProfilePanelClassName).toContain('opacity-100')
  })

  it('prefills create-contact values from the support customer identity', () => {
    expect(customerProfileInitialContactValues(conversation)).toMatchObject({
      first_name: 'Fallback',
      last_name: 'Name',
      email: 'fallback@example.com',
      lifecycle_stage: 'subscriber',
      lead_status: 'new',
      source: 'support',
    })
  })
})
