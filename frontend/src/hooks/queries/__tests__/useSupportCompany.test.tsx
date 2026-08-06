// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { queryKeys } from '@/lib/queryKeys'

const captured = {
  mutation: null as ReturnType<
    typeof import('@/hooks/queries/useSupport').useUpdateConversationCRMCompany
  > | null,
}

vi.mock('@/lib/services/supportService', () => ({
  supportService: {
    updateConversationCRMCompany: vi.fn(),
  },
}))

import { useUpdateConversationCRMCompany } from '@/hooks/queries/useSupport'
import { supportService } from '@/lib/services/supportService'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function Harness() {
  captured.mutation = useUpdateConversationCRMCompany('workspace-1')
  return null
}

describe('useUpdateConversationCRMCompany', () => {
  afterEach(() => {
    captured.mutation = null
    vi.clearAllMocks()
  })

  it('updates the company and invalidates conversation company context', async () => {
    vi.mocked(supportService.updateConversationCRMCompany).mockResolvedValue({
      data: { id: 'conversation-1', crm_company_id: 'company-1' },
      error: null,
      status: 200,
    } as never)

    const client = new QueryClient()
    const conversationKey = queryKeys.support.conversation('workspace-1', 'conversation-1')
    const visitorKey = queryKeys.support.visitorContext('workspace-1', 'conversation-1')
    const associationsKey = queryKeys.support.conversationAssociations(
      'workspace-1',
      'conversation-1',
    )
    client.setQueryData(conversationKey, {})
    client.setQueryData(visitorKey, {})
    client.setQueryData(associationsKey, {})

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      await captured.mutation?.mutateAsync({
        conversationId: 'conversation-1',
        companyId: 'company-1',
      })
    })

    expect(supportService.updateConversationCRMCompany).toHaveBeenCalledWith(
      'workspace-1',
      'conversation-1',
      { crm_company_id: 'company-1' },
    )
    expect(client.getQueryState(conversationKey)?.isInvalidated).toBe(true)
    expect(client.getQueryState(visitorKey)?.isInvalidated).toBe(true)
    expect(client.getQueryState(associationsKey)?.isInvalidated).toBe(true)

    act(() => root.unmount())
    container.remove()
  })
})
