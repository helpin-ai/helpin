// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { SupportSLAPolicy } from '@/lib/pmTypes'

const createPolicy = vi.fn()
const updatePolicy = vi.fn()
const deletePolicy = vi.fn()
const reorder = vi.fn()
let policies: SupportSLAPolicy[] = []

vi.mock('@/hooks/queries/useSupport', () => ({
  useInboxScopes: () => ({ data: { mailboxes: [{ id: 'mb-1', name: 'Billing' }] } }),
  useChatSettings: () => ({ data: { settings: { business_hours_enabled: false } } }),
}))
vi.mock('@/hooks/queries/useSupportOperations', () => ({
  useSLAPolicies: () => ({ data: policies, isLoading: false, error: null }),
  useCreateSLAPolicy: () => ({ mutateAsync: createPolicy, isPending: false }),
  useUpdateSLAPolicy: () => ({ mutate: updatePolicy, mutateAsync: updatePolicy, isPending: false }),
  useDeleteSLAPolicy: () => ({ mutate: deletePolicy, isPending: false }),
  useReorderSLAPolicies: () => ({ mutate: reorder, isPending: false }),
}))
vi.mock('@/components/ui/confirm-dialog', () => ({ useConfirm: () => async () => true }))

import { SupportSLAPolicies } from '../SupportSLAPolicies'

Object.assign(globalThis, {
  IS_REACT_ACT_ENVIRONMENT: true,
  ResizeObserver: class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
})

function policy(overrides: Partial<SupportSLAPolicy>): SupportSLAPolicy {
  return {
    id: 'p1', workspace_id: 'ws', name: 'Urgent', enabled: true, position: 0, priorities: ['urgent'], mailbox_ids: [], channels: [],
    first_response_minutes: 60, next_response_minutes: null, resolution_minutes: 1440, business_hours: false, pause_while_waiting: true,
    active_since: '2026-10-01T00:00:00Z', created_at: '2026-10-01T00:00:00Z', updated_at: '2026-10-01T00:00:00Z',
    ...overrides,
  }
}

function setInput(element: HTMLInputElement, value: string) {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(element, value)
  element.dispatchEvent(new Event('input', { bubbles: true }))
}

function button(text: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find((element) => element.textContent?.trim() === text || element.getAttribute('aria-label') === text) as HTMLButtonElement | undefined
}

describe('SupportSLAPolicies', () => {
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

  const render = (editable = true) => act(() => root.render(<TooltipProvider><SupportSLAPolicies workspaceId="ws" editable={editable} /></TooltipProvider>))

  it('lists policies in order with their targets and conditions', () => {
    policies = [policy({}), policy({ id: 'p2', name: 'Billing email', priorities: [], mailbox_ids: ['mb-1'], channels: ['email'], enabled: false, first_response_minutes: 30, resolution_minutes: null })]
    render()
    const rows = container.querySelectorAll('[data-testid="sla-policy"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].textContent).toContain('First response 1h · Resolution 1d — Urgent')
    expect(rows[1].textContent).toContain('Off')
    expect(rows[1].textContent).toContain('Billing · Email')
    act(() => button('Move Billing email up')?.click())
    expect(reorder).toHaveBeenCalledWith(['p2', 'p1'])
    expect(button('Move Urgent up')?.disabled).toBe(true)
  })

  it('validates and creates a policy', async () => {
    policies = []
    createPolicy.mockResolvedValue({})
    render()
    act(() => button('New policy')?.click())
    const submit = button('Create policy')!
    await act(async () => { submit.click() })
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('Give the policy a name.')
    expect(createPolicy).not.toHaveBeenCalled()

    act(() => setInput(document.getElementById('sla-policy-name') as HTMLInputElement, 'Urgent customers'))
    act(() => button('Urgent')?.click())
    act(() => setInput(document.getElementById('sla-first-amount') as HTMLInputElement, '30'))
    await act(async () => { button('Create policy')!.click() })
    expect(createPolicy).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Urgent customers', priorities: ['urgent'], first_response_minutes: 1800,
    }))
  })

  it('hides editing for read-only teammates', () => {
    policies = [policy({})]
    render(false)
    expect(button('New policy')).toBeUndefined()
    expect(button('Edit Urgent')).toBeUndefined()
  })

  it('confirms before deleting', async () => {
    policies = [policy({})]
    render()
    await act(async () => { button('Delete Urgent')?.click() })
    expect(deletePolicy).toHaveBeenCalledWith('p1')
  })
})
