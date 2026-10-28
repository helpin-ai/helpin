// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { SupportCompanySLA, SupportSLATier } from '@/lib/pmTypes'

const setTier = vi.fn()
const customize = vi.fn()
const removeCustom = vi.fn()
const confirm = vi.fn()
let sla: SupportCompanySLA | undefined
let tiers: SupportSLATier[] = []

vi.mock('@/hooks/queries/useSupportSLATiers', () => ({
  useCompanySLA: () => ({ data: sla, isLoading: false, error: null }),
  useSLATiers: () => ({ data: tiers }),
  useSetCompanySLATier: () => ({ mutate: setTier, isPending: false }),
  useCustomizeCompanySLA: () => ({ mutate: customize, isPending: false }),
  useRemoveCompanySLA: () => ({ mutate: removeCustom, isPending: false }),
}))
vi.mock('@/components/ui/confirm-dialog', () => ({ useConfirm: () => confirm }))
vi.mock('sonner', () => ({ toast: { success: vi.fn() } }))

import { CompanySLASection } from '../CompanySLASection'

Object.assign(globalThis, {
  IS_REACT_ACT_ENVIRONMENT: true,
  ResizeObserver: class { observe() {} unobserve() {} disconnect() {} },
})
Element.prototype.scrollIntoView = () => {}

const gold = { id: 'tier-gold', workspace_id: 'ws', name: 'Gold', description: '', position: 0, company_count: 1, policy_count: 1, created_at: '', updated_at: '' }
const goldPolicy = { id: 'p1', name: 'Gold urgent', custom: false, priorities: ['urgent' as const], first_response_minutes: 30, resolution_minutes: 1440, business_hours: true }

function button(text: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find((element) => element.textContent?.trim() === text || element.getAttribute('aria-label') === text) as HTMLButtonElement | undefined
}

describe('CompanySLASection', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    vi.clearAllMocks()
    tiers = [gold]
    sla = { company_id: 'co-1', tier_id: 'tier-gold', tier_name: 'Gold', has_custom: false, policies: [goldPolicy] }
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })
  afterEach(() => {
    act(() => root.unmount())
    document.body.innerHTML = ''
  })

  const render = (canManage = true) => act(() => root.render(
    <TooltipProvider><CompanySLASection workspaceId="ws" workspaceSlug="acme" companyId="co-1" companyName="Acme" canManage={canManage} /></TooltipProvider>,
  ))

  it('shows the tier and the policies that apply, in order', () => {
    render()
    expect(container.textContent).toContain('Support SLA')
    expect(container.textContent).toContain('Gold urgent')
    expect(container.textContent).toContain('Urgent: First response 30m · Resolution 1d (business hours)')
    expect(container.querySelector('a[href="/w/acme/settings/support-sla"]')).not.toBeNull()
  })

  it('changes the tier from the rail', () => {
    render()
    act(() => button('Gold')?.click())
    const none = Array.from(document.querySelectorAll('[cmdk-item]')).find((item) => item.textContent?.includes('No tier')) as HTMLElement
    act(() => none.click())
    expect(setTier).toHaveBeenCalledWith({ companyIds: ['co-1'], tierId: null })
  })

  it('creates a custom SLA, or removes one after confirming', async () => {
    render()
    act(() => button('Customize for this company')?.click())
    expect(customize).toHaveBeenCalledWith('co-1', expect.anything())

    sla = { ...sla!, has_custom: true, policies: [{ ...goldPolicy, id: 'c1', name: 'Acme · Gold urgent', custom: true }, goldPolicy] }
    confirm.mockResolvedValue(true)
    render()
    expect(container.textContent).toContain('Custom')
    await act(async () => { button('Remove custom SLA')?.click() })
    expect(removeCustom).toHaveBeenCalledWith('co-1')
  })

  it('is read-only without support admin access', () => {
    render(false)
    expect(container.textContent).toContain('Gold')
    expect(button('Customize for this company')).toBeUndefined()
    expect(button('Gold')).toBeUndefined()
  })

  it('stays hidden while the workspace has no SLA setup', () => {
    tiers = []
    sla = { company_id: 'co-1', tier_id: null, tier_name: null, has_custom: false, policies: [] }
    render()
    expect(container.textContent).toBe('')
  })
})
