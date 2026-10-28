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
        agent_enrichment: {
          headquarters: { value: 'New York', source_url: 'https://example.com/internal-source' },
        },
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

const companySLA = vi.hoisted(() => vi.fn(() => ({ data: undefined as unknown })))
vi.mock('@/hooks/queries/useSupportSLATiers', () => ({
  useCompanySLA: (...args: unknown[]) => companySLA(...args),
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
    window.localStorage.clear()
    mockState.canEdit = true
    mockState.isLoading = false
    mockState.context.company_context_status = 'ok'
    mockState.context.company = {
      id: 'company-1', display_id: 'COM-1', name: 'Acme', domain: 'acme.example',
      custom_properties: {
        plan: 'enterprise', subscription_status: 'active', support_tier: 'priority',
        seats_used: 12, seats_total: 25, renewal_date: '2027-01-15', mrr: 4200,
        currency: 'USD', priority_support: true, sdk_company_id: 'hidden',
        agent_enrichment: {
          headquarters: { value: 'New York', source_url: 'https://example.com/internal-source' },
        },
      },
      updated_at: '2026-08-05T12:00:00Z',
    }
    mockState.context.company_options = [{ id: 'company-1', display_id: 'COM-1', name: 'Acme', domain: 'acme.example' }]
    vi.clearAllMocks()
    companySLA.mockReturnValue({ data: undefined })
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
    companySLA.mockReturnValue({ data: { company_id: 'company-1', tier_id: 't1', tier_name: 'Gold', has_custom: false, policies: [] } })
    const { container, root } = renderCompanyDetails()

    expect(companySLA).toHaveBeenLastCalledWith('ws-1', 'company-1', true)
    expect(container.textContent).toContain('SLA tierGold')
    expect(container.textContent).toContain('Company details')
    expect(container.textContent).toContain('Acme')
    expect(container.textContent).toContain('Enterprise')
    expect(container.textContent).toContain('Active')
    expect(container.textContent).toContain('12 / 25')
    expect(container.textContent).toContain('$4,200')
    expect(container.textContent).toContain('Yes')
    expect(container.textContent).not.toContain('hidden')
    expect(container.textContent).not.toContain('Agent Enrichment')
    expect(container.textContent).not.toContain('internal-source')

    act(() => root.unmount())
  })

  it('marks a custom SLA next to the tier', () => {
    companySLA.mockReturnValue({ data: { company_id: 'company-1', tier_id: 't1', tier_name: 'Gold', has_custom: true, policies: [] } })
    const { container, root } = renderCompanyDetails()
    expect(container.textContent).toContain('SLA tierGold, custom SLA')
    act(() => root.unmount())
  })

  it('hides company editing controls from read-only agents', () => {
    mockState.canEdit = false
    const { container, root } = renderCompanyDetails()

    expect(container.textContent).toContain('Acme')
    expect(container.querySelector('[aria-label="Company options"]')).toBeNull()
    expect(container.textContent).not.toContain('Link another company')
    expect(container.textContent).not.toContain('Clear')

    act(() => root.unmount())
  })

  it('opens the company in CRM in a new tab and keeps switching in the field itself', () => {
    const { container, root } = renderCompanyDetails()
    const link = container.querySelector('a[aria-label="Open Acme in CRM"]') as HTMLAnchorElement | null

    expect(link?.getAttribute('href')).toBe('/w/acme-workspace/crm/companies/company-1')
    expect(link?.getAttribute('target')).toBe('_blank')
    expect(container.querySelector('[aria-label="Company options"]')).not.toBeNull()
    expect(container.querySelector('select')).toBeNull()
    expect(container.textContent).not.toContain('Link another company')

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

    expect(container.textContent).toContain('Select a company')
    expect(container.querySelector('select')).toBeNull()

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
