// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { SupportSLATier } from '@/lib/pmTypes'

const createTier = vi.fn()
const updateTier = vi.fn()
const deleteTier = vi.fn()
const reorder = vi.fn()
const confirm = vi.fn()
let tiers: SupportSLATier[] = []

vi.mock('@/hooks/queries/useSupportSLATiers', () => ({
  useSLATiers: () => ({ data: tiers, isLoading: false, error: null }),
  useCreateSLATier: () => ({ mutateAsync: createTier, isPending: false }),
  useUpdateSLATier: () => ({ mutateAsync: updateTier, isPending: false }),
  useDeleteSLATier: () => ({ mutate: deleteTier, isPending: false }),
  useReorderSLATiers: () => ({ mutate: reorder, isPending: false }),
}))
vi.mock('@/components/ui/confirm-dialog', () => ({ useConfirm: () => confirm }))

import { SupportSLATiers } from '../SupportSLATiers'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function tier(overrides: Partial<SupportSLATier>): SupportSLATier {
  return {
    id: 't1', workspace_id: 'ws', name: 'Gold', description: 'Enterprise', position: 0, company_count: 3, policy_count: 1,
    created_at: '2026-10-01T00:00:00Z', updated_at: '2026-10-01T00:00:00Z', ...overrides,
  }
}

function setInput(element: HTMLInputElement, value: string) {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(element, value)
  element.dispatchEvent(new Event('input', { bubbles: true }))
}

function button(text: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find((element) => element.textContent?.trim() === text || element.getAttribute('aria-label') === text) as HTMLButtonElement | undefined
}

describe('SupportSLATiers', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    vi.clearAllMocks()
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })
  afterEach(() => {
    act(() => root.unmount())
    document.body.innerHTML = ''
  })

  const render = (editable = true) => act(() => root.render(<TooltipProvider><SupportSLATiers workspaceId="ws" editable={editable} /></TooltipProvider>))

  it('lists tiers with their usage and reorders them', () => {
    tiers = [tier({}), tier({ id: 't2', name: 'Silver', description: '', company_count: 1, policy_count: 0 })]
    render()
    const rows = container.querySelectorAll('[data-testid="sla-tier"]')
    expect(rows[0].textContent).toContain('Enterprise · 3 companies · 1 policy')
    expect(rows[1].textContent).toContain('1 company · 0 policies')
    act(() => button('Move Silver up')?.click())
    expect(reorder).toHaveBeenCalledWith(['t2', 't1'])
  })

  it('creates a tier', async () => {
    tiers = []
    createTier.mockResolvedValue({})
    render()
    expect(container.textContent).toContain('No tiers yet')
    act(() => button('New tier')?.click())
    expect(button('Create tier')?.disabled).toBe(true)
    act(() => setInput(document.getElementById('sla-tier-name') as HTMLInputElement, '  Gold '))
    act(() => setInput(document.getElementById('sla-tier-description') as HTMLInputElement, 'Enterprise plan'))
    await act(async () => { button('Create tier')!.click() })
    expect(createTier).toHaveBeenCalledWith({ name: 'Gold', description: 'Enterprise plan' })
    expect(document.getElementById('sla-tier-name')).toBeNull()
  })

  it('explains what deleting a tier changes', async () => {
    tiers = [tier({})]
    confirm.mockResolvedValue(true)
    render()
    await act(async () => { button('Delete Gold')?.click() })
    expect(confirm).toHaveBeenCalledWith(expect.objectContaining({
      description: expect.stringContaining('3 companies will have no tier, and 1 policy will stop matching it, and a policy for this tier alone is turned off'),
    }))
    expect(deleteTier).toHaveBeenCalledWith('t1')
  })

  it('is read-only without support admin access', () => {
    tiers = [tier({})]
    render(false)
    expect(button('New tier')).toBeUndefined()
    expect(button('Edit Gold')).toBeUndefined()
  })
})
