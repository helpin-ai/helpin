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
