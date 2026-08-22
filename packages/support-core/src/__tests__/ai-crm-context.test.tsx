import React from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import { supportQueryKeys } from '../support-query-keys'
import { configureSupportApi } from '../support-service'
import {
  useRewriteSupportDraft,
  useUpdateConversationCRMContact,
  useUpdateConversationCRMCompany,
  useUpdateConversationCustomerName,
  useUpdateConversationEmailRecipients,
} from '../use-support-core'

const workspaceId = 'ws-1'
const conversationId = 'conv-1'

function makeApi() {
  return {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    patch: vi.fn(),
    del: vi.fn(),
  }
}

function wrapper(queryClient: QueryClient) {
  return function TestWrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  }
}

describe('support AI and CRM context mutations', () => {
  const api = makeApi()

  beforeEach(() => {
    vi.clearAllMocks()
    configureSupportApi(api)
  })

  test('new-conversation rewrite uses the workspace rewrite-draft endpoint', async () => {
    api.post.mockResolvedValue({
      data: { content: 'Hello there', operation: 'rephrase', provider: 'test', model: 'test' },
      error: null,
    })
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const { result } = renderHook(() => useRewriteSupportDraft(workspaceId, null), {
      wrapper: wrapper(queryClient),
    })

    await act(async () => {
      await result.current.mutateAsync({ content: 'hi', operation: 'rephrase' })
    })

    expect(api.post).toHaveBeenCalledWith(
      `/support/inbox/rewrite-draft?workspace_id=${workspaceId}`,
      { content: 'hi', operation: 'rephrase' },
    )
  })

  test('company selection sends the exact crm_company_id payload and refreshes context', async () => {
    api.put.mockResolvedValue({ data: { id: conversationId }, error: null })
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useUpdateConversationCRMCompany(workspaceId), {
      wrapper: wrapper(queryClient),
    })

    await act(async () => {
      await result.current.mutateAsync({ conversationId, companyId: 'company-1' })
    })

    expect(api.put).toHaveBeenCalledWith(
      `/support/inbox/conversations/${conversationId}/crm-company?workspace_id=${workspaceId}`,
      { crm_company_id: 'company-1' },
    )
    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: supportQueryKeys.visitorContext(workspaceId, conversationId),
      })
    })
  })

  test('customer-name edit sends the existing customer_name contract', async () => {
    api.put.mockResolvedValue({ data: { id: conversationId }, error: null })
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const { result } = renderHook(() => useUpdateConversationCustomerName(workspaceId), {
      wrapper: wrapper(queryClient),
    })

    await act(async () => {
      await result.current.mutateAsync({ conversationId, customerName: 'Ada Lovelace' })
    })

    expect(api.put).toHaveBeenCalledWith(
      `/support/inbox/conversations/${conversationId}/customer-name?workspace_id=${workspaceId}`,
      { customer_name: 'Ada Lovelace' },
    )
  })

  test('CRM contact relinking sends the exact crm_contact_id payload', async () => {
    api.put.mockResolvedValue({ data: { id: conversationId }, error: null })
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const { result } = renderHook(() => useUpdateConversationCRMContact(workspaceId), {
      wrapper: wrapper(queryClient),
    })

    await act(async () => {
      await result.current.mutateAsync({ conversationId, contactId: 'contact-2' })
    })

    expect(api.put).toHaveBeenCalledWith(
      `/support/inbox/conversations/${conversationId}/crm-contact?workspace_id=${workspaceId}`,
      { crm_contact_id: 'contact-2' },
    )
  })

  test('Cc updates use the existing email recipient contract', async () => {
    api.put.mockResolvedValue({ data: { id: conversationId }, error: null })
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    const { result } = renderHook(() => useUpdateConversationEmailRecipients(workspaceId), {
      wrapper: wrapper(queryClient),
    })

    await act(async () => {
      await result.current.mutateAsync({
        conversationId,
        payload: { cc_emails: ['finance@example.com'] },
      })
    })

    expect(api.put).toHaveBeenCalledWith(
      `/support/inbox/conversations/${conversationId}/email-recipients?workspace_id=${workspaceId}`,
      { cc_emails: ['finance@example.com'] },
    )
  })
})
