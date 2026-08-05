// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import { SidebarCompanyDetails } from '../SidebarCompanyDetails'

const mockState = vi.hoisted(() => ({
  canEdit: true,
  isLoading: false,
  refetch: vi.fn(),
  mutate: vi.fn(),
  context: {
    company_context_status: 'ok' as 'ok' | 'unlinked' | 'error',
    company: {
      id: 'company-1', display_id: 'COM-1', name: 'Acme', domain: 'acme.example',
      custom_properties: {
        plan: 'enterprise', subscription_status: 'active', support_tier: 'priority',
        seats_used: 12, seats_total: 25, renewal_date: '2027-01-15', mrr: 4200,
        currency: 'USD', priority_support: true, sdk_company_id: 'hidden',
      },
      updated_at: '2026-08-05T12:00:00Z',
    } as Record<string, any> | null,
    company_options: [{ id: 'company-1', display_id: 'COM-1', name: 'Acme', domain: 'acme.example' }],
  },
}))

vi.mock('@/hooks/queries/useSupport', () => ({
  useVisitorContext: () => ({
    data: mockState.context,
    isLoading: mockState.isLoading,
    refetch: mockState.refetch,
  }),
  useUpdateConversationCRMCompany: () => ({ mutate: mockState.mutate, isPending: false }),
}))

vi.mock('@/hooks/queries', () => ({
  useWorkspaceAccess: () => ({ data: {} }),
  usePermissions: () => ({ has: () => mockState.canEdit }),
}))

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: any) => unknown) => selector({ currentWorkspace: { slug: 'acme-workspace' } }),
}))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('SidebarCompanyDetails', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    mockState.canEdit = true
    mockState.isLoading = false
    mockState.context.company_context_status = 'ok'
    mockState.context.company = {
      id: 'company-1', display_id: 'COM-1', name: 'Acme', domain: 'acme.example',
      custom_properties: {
        plan: 'enterprise', subscription_status: 'active', support_tier: 'priority',
        seats_used: 12, seats_total: 25, renewal_date: '2027-01-15', mrr: 4200,
        currency: 'USD', priority_support: true, sdk_company_id: 'hidden',
      },
      updated_at: '2026-08-05T12:00:00Z',
    }
    mockState.context.company_options = [{ id: 'company-1', display_id: 'COM-1', name: 'Acme', domain: 'acme.example' }]
    vi.clearAllMocks()
  })

  function renderCompanyDetails() {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    act(() => {
      root.render(<TooltipProvider><SidebarCompanyDetails workspaceId="ws-1" conversationId="conv-1" /></TooltipProvider>)
    })
    return { container, root }
  }

  it('shows live subscription details and hides technical SDK attributes', () => {
    const { container, root } = renderCompanyDetails()

    expect(container.textContent).toContain('Company Details')
    expect(container.textContent).toContain('Acme')
    expect(container.textContent).toContain('Enterprise')
    expect(container.textContent).toContain('Active')
    expect(container.textContent).toContain('12 / 25')
    expect(container.textContent).toContain('$4,200')
    expect(container.textContent).toContain('Yes')
    expect(container.textContent).not.toContain('hidden')

    act(() => root.unmount())
  })

  it('hides company editing controls from read-only agents', () => {
    mockState.canEdit = false
    const { container, root } = renderCompanyDetails()

    expect(container.textContent).toContain('Acme')
    expect(container.textContent).not.toContain('Link another company')
    expect(container.textContent).not.toContain('Clear')

    act(() => root.unmount())
  })

  it('asks agents to select rather than guessing when several memberships exist', () => {
    mockState.context.company_context_status = 'unlinked'
    mockState.context.company = null
    mockState.context.company_options = [
      { id: 'company-1', display_id: 'COM-1', name: 'Acme', domain: 'acme.example' },
      { id: 'company-2', display_id: 'COM-2', name: 'Beta', domain: 'beta.example' },
    ]
    const { container, root } = renderCompanyDetails()

    expect(container.textContent).toContain('Select the company for this conversation.')
    expect(container.querySelector('select[aria-label="Select conversation company"]')).not.toBeNull()
    expect(container.textContent).toContain('Link a company')

    act(() => root.unmount())
  })

  it('shows a retry state when optional company context fails', () => {
    mockState.context.company_context_status = 'error'
    const { container, root } = renderCompanyDetails()
    const retry = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Retry'))

    expect(container.textContent).toContain('Company details could not be loaded.')
    act(() => retry?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    expect(mockState.refetch).toHaveBeenCalled()

    act(() => root.unmount())
  })
})
