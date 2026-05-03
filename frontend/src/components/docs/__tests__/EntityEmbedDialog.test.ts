import { beforeEach, describe, expect, it, vi } from 'vitest'

const serviceMocks = vi.hoisted(() => ({
  search: vi.fn(),
  listConversations: vi.fn(),
  listContacts: vi.fn(),
  listCompanies: vi.fn(),
  listDeals: vi.fn(),
}))

vi.mock('@/lib/services/searchService', () => ({
  searchService: {
    search: (...args: unknown[]) => serviceMocks.search(...args),
  },
}))

vi.mock('@/lib/services/supportService', () => ({
  supportService: {
    listConversations: (...args: unknown[]) => serviceMocks.listConversations(...args),
  },
}))

vi.mock('@/lib/services/crmService', () => ({
  crmContactService: {
    list: (...args: unknown[]) => serviceMocks.listContacts(...args),
  },
  crmCompanyService: {
    list: (...args: unknown[]) => serviceMocks.listCompanies(...args),
  },
  crmDealService: {
    list: (...args: unknown[]) => serviceMocks.listDeals(...args),
  },
}))

import { searchEntityEmbedItems } from '../EntityEmbedDialog'

describe('searchEntityEmbedItems', () => {
  beforeEach(() => {
    serviceMocks.search.mockReset().mockResolvedValue({
      data: { tasks: [], epics: [], sprints: [], objectives: [], members: [], documents: [] },
      error: null,
    })
    serviceMocks.listConversations.mockReset().mockResolvedValue({ data: { data: [] }, error: null })
    serviceMocks.listContacts.mockReset().mockResolvedValue({ data: { data: [] }, error: null })
    serviceMocks.listCompanies.mockReset().mockResolvedValue({ data: { data: [] }, error: null })
    serviceMocks.listDeals.mockReset().mockResolvedValue({ data: { data: [] }, error: null })
  })

  it('searches only support conversations for fixed conversation embeds', async () => {
    serviceMocks.listConversations.mockResolvedValue({
      data: {
        data: [
          {
            id: 'conv-1',
            display_id: 42,
            subject: 'Billing question',
            status: 'open',
            customer_email: 'buyer@example.com',
            mailbox_name: 'Support',
          },
        ],
      },
      error: null,
    })

    const result = await searchEntityEmbedItems('ws-1', 'billing', 'support_conversation')

    expect(serviceMocks.listConversations).toHaveBeenCalledWith('ws-1', { search: 'billing', per_page: 10 })
    expect(serviceMocks.search).not.toHaveBeenCalled()
    expect(serviceMocks.listContacts).not.toHaveBeenCalled()
    expect(serviceMocks.listCompanies).not.toHaveBeenCalled()
    expect(serviceMocks.listDeals).not.toHaveBeenCalled()
    expect(result.items).toEqual([
      expect.objectContaining({
        entityType: 'support_conversation',
        entityId: 'conv-1',
        title: 'Billing question',
      }),
    ])
  })

  it('searches only contacts for fixed contact embeds', async () => {
    serviceMocks.listContacts.mockResolvedValue({
      data: {
        data: [
          {
            id: 'contact-1',
            display_id: 7,
            first_name: 'Ada',
            last_name: 'Lovelace',
            email: 'ada@example.com',
            job_title: 'CTO',
            lifecycle_stage: 'lead',
          },
        ],
      },
      error: null,
    })

    const result = await searchEntityEmbedItems('ws-1', 'ada', 'contact')

    expect(serviceMocks.listContacts).toHaveBeenCalledWith('ws-1', { search: 'ada', per_page: 10 })
    expect(serviceMocks.search).not.toHaveBeenCalled()
    expect(serviceMocks.listConversations).not.toHaveBeenCalled()
    expect(serviceMocks.listCompanies).not.toHaveBeenCalled()
    expect(serviceMocks.listDeals).not.toHaveBeenCalled()
    expect(result.items).toEqual([
      expect.objectContaining({
        entityType: 'contact',
        entityId: 'contact-1',
        title: 'Ada Lovelace',
      }),
    ])
  })

  it('keeps generic entity search broad', async () => {
    await searchEntityEmbedItems('ws-1', 'alpha')

    expect(serviceMocks.search).toHaveBeenCalledWith('ws-1', 'alpha')
    expect(serviceMocks.listConversations).toHaveBeenCalledWith('ws-1', { search: 'alpha', per_page: 6 })
    expect(serviceMocks.listContacts).toHaveBeenCalledWith('ws-1', { search: 'alpha', per_page: 4 })
    expect(serviceMocks.listCompanies).toHaveBeenCalledWith('ws-1', { search: 'alpha', per_page: 4 })
    expect(serviceMocks.listDeals).toHaveBeenCalledWith('ws-1', { search: 'alpha', per_page: 4 })
  })
})
