import { afterEach, describe, expect, it, vi } from 'vitest'

import { supportService } from '../supportService'

describe('supportService conversation company context', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('updates the linked company and supports clearing it', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: 'conversation-1', crm_company_id: null }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await supportService.updateConversationCRMCompany('workspace-1', 'conversation-1', {
      crm_company_id: null,
    })

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining(
        '/support/inbox/conversations/conversation-1/crm-company?workspace_id=workspace-1',
      ),
      expect.objectContaining({
        method: 'PUT',
        credentials: 'include',
        body: JSON.stringify({ crm_company_id: null }),
      }),
    )
  })
})

describe('supportService personal read state', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('sends the rendered customer message cursor', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ conversation_id: 'conversation-1', unread_customer_message_count: 0 }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await supportService.markConversationRead('workspace-1', 'conversation-1', 'customer-message-7')

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/support/inbox/conversations/conversation-1/read?workspace_id=workspace-1'),
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ through_message_id: 'customer-message-7' }),
      }),
    )
  })
})
